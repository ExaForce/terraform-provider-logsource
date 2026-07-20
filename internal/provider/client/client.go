package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	Endpoint    string
	APIToken    string // X-EXF-API-TOKEN header
	IDToken     string // X-EXF-ID-TOKEN cookie (legacy cookie auth fallback)
	AccessToken string // X-EXF-ACCESS-TOKEN cookie (legacy cookie auth fallback)
	Session     string // session cookie (legacy cookie auth fallback)
	Project     string
	HTTPClient  *http.Client

	csrfMu    sync.Mutex
	csrfToken string

	eksClustersMu    sync.Mutex
	eksClusterCache  []EKSCluster
}

func New(endpoint, apiToken, idToken, accessToken, session, project string) *Client {
	return &Client{
		Endpoint:    endpoint,
		APIToken:    apiToken,
		IDToken:     idToken,
		AccessToken: accessToken,
		Session:     session,
		Project:     project,
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// addAuth adds auth headers/cookies to the request.
func (c *Client) addAuth(req *http.Request) {
	if c.APIToken != "" {
		req.Header.Set("X-EXF-API-TOKEN", c.APIToken)
	} else {
		req.AddCookie(&http.Cookie{Name: "X-EXF-ID-TOKEN", Value: c.IDToken})
		req.AddCookie(&http.Cookie{Name: "X-EXF-ACCESS-TOKEN", Value: c.AccessToken})
		if c.Session != "" {
			req.AddCookie(&http.Cookie{Name: "session", Value: c.Session})
		}
	}
}

// fetchCSRF calls the whoami endpoint (CSRF-exempt) to obtain a CSRF token.
// Result is cached on the client; call invalidateCSRF() to force a refresh.
func (c *Client) fetchCSRF(ctx context.Context) (string, error) {
	c.csrfMu.Lock()
	defer c.csrfMu.Unlock()

	if c.csrfToken != "" {
		return c.csrfToken, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint+"/ae/rbac/system/whoami", nil)
	if err != nil {
		return "", fmt.Errorf("create whoami request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	c.addAuth(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whoami request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read whoami response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("whoami returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse whoami response: %w", err)
	}
	if result.CSRF == "" {
		return "", fmt.Errorf("whoami response missing csrf field")
	}

	c.csrfToken = result.CSRF
	return c.csrfToken, nil
}

func (c *Client) invalidateCSRF() {
	c.csrfMu.Lock()
	c.csrfToken = ""
	c.csrfMu.Unlock()
}

// do executes an HTTP request. For cookie auth, retries once after refreshing
// the CSRF token on 401 (tokens expire during long applies).
func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	return c.doAttempt(ctx, method, path, body, true)
}

func (c *Client) doAttempt(ctx context.Context, method, path string, body any, allowRetry bool) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	fullURL := c.Endpoint + path

	// Cookie auth requires a CSRF token appended as ?csrf=<token>.
	// API token auth skips CSRF entirely.
	if c.APIToken == "" {
		csrf, err := c.fetchCSRF(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetch CSRF token: %w", err)
		}
		if len(path) > 0 && path[len(path)-1] == '/' {
			fullURL = fullURL[:len(fullURL)-1]
		}
		fullURL = fullURL + "?csrf=" + csrf
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.addAuth(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	// On 401 in cookie mode, CSRF may have expired mid-apply. Refresh and retry once.
	if resp.StatusCode == http.StatusUnauthorized && c.APIToken == "" && allowRetry {
		resp.Body.Close()
		c.invalidateCSRF()
		return c.doAttempt(ctx, method, path, body, false)
	}

	return resp, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) (int, error) {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// Log source types — matches actual CloudScout API response format.
// List endpoint returns []LogSource; single returns LogSource.
// name lives in metadata.name, not at the top level.

type LogSourceSpec struct {
	LogSourceType string `json:"log_source_type"`
	EksArn        string `json:"eks_arn,omitempty"`
	SqsURL        string `json:"sqs_url,omitempty"`
	S3BucketArn   string `json:"s3_bucket_arn,omitempty"`
	S3KeyPrefix   string `json:"s3_key_prefix,omitempty"`
	HomeRegion    string `json:"home_region,omitempty"`
	EcrArn        string `json:"ecr_arn,omitempty"`
}

type LogSourceMetadata struct {
	Name        string `json:"name"`
	Project     string `json:"project"`
	Description string `json:"description"`
}

type LogSource struct {
	Spec     LogSourceSpec     `json:"spec"`
	Metadata LogSourceMetadata `json:"metadata"`
}

func (ls *LogSource) Name() string { return ls.Metadata.Name }

type CreateLogSourceRequest struct {
	Name string        `json:"name,omitempty"`
	Spec LogSourceSpec `json:"spec"`
}

func (c *Client) CreateLogSource(ctx context.Context, name string, spec LogSourceSpec) (*LogSource, error) {
	req := CreateLogSourceRequest{Name: name, Spec: spec}
	var result LogSource
	if _, err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/cs/api/%s/aws_logsources", c.Project), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetLogSource(ctx context.Context, name string) (*LogSource, int, error) {
	var result LogSource
	status, err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/cs/api/%s/aws_logsource/%s", c.Project, name), nil, &result)
	if err != nil {
		return nil, status, err
	}
	return &result, status, nil
}

func (c *Client) ListLogSources(ctx context.Context) ([]LogSource, error) {
	var result []LogSource
	if _, err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/cs/api/%s/aws_logsources", c.Project), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// EKSCluster represents a discovered EKS cluster from CloudScout.
type EKSCluster struct {
	Name           string `json:"name"`
	ARN            string `json:"arn"`
	AccountID      string `json:"account_id"`
	Region         string `json:"region"`
	ExabotRoleArn  string `json:"exabot_role_arn"`
	ExabotSqsURL   string `json:"exabot_sqs_url"`
	BucketID       string `json:"bucket_id"`
}

type eksClusterResponse struct {
	Spec struct {
		Name          string `json:"name"`
		ARN           string `json:"arn"`
		AccountID     string `json:"account_id"`
		Region        string `json:"region"`
		ExabotRoleArn string `json:"exabot_role_arn"`
		ExabotSqsURL  string `json:"exabot_sqs_url"`
		BucketID      string `json:"bucket_id"`
	} `json:"spec"`
}

// ListEKSClusters returns all EKS clusters discovered by CloudScout.
// Results are cached for the lifetime of the client (one terraform apply) so that
// 43 concurrent resource Creates don't each issue a full list call.
func (c *Client) ListEKSClusters(ctx context.Context) ([]EKSCluster, error) {
	c.eksClustersMu.Lock()
	defer c.eksClustersMu.Unlock()

	if c.eksClusterCache != nil {
		return c.eksClusterCache, nil
	}

	var raw []eksClusterResponse
	if _, err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/cs/api/%s/aws_eks_clusters", c.Project), nil, &raw); err != nil {
		return nil, fmt.Errorf("list EKS clusters: %w", err)
	}
	clusters := make([]EKSCluster, 0, len(raw))
	for _, r := range raw {
		clusters = append(clusters, EKSCluster{
				Name:          r.Spec.Name,
				ARN:           r.Spec.ARN,
				AccountID:     r.Spec.AccountID,
				Region:        r.Spec.Region,
				ExabotRoleArn: r.Spec.ExabotRoleArn,
				ExabotSqsURL:  r.Spec.ExabotSqsURL,
				BucketID:      r.Spec.BucketID,
			})
	}
	c.eksClusterCache = clusters
	return c.eksClusterCache, nil
}

// LookupEKSClusterByName finds a discovered EKS cluster by its name and returns
// the cluster ARN. Returns an error if the cluster is not found or not yet
// discovered by CloudScout.
func (c *Client) LookupEKSClusterByName(ctx context.Context, clusterName string) (*EKSCluster, error) {
	clusters, err := c.ListEKSClusters(ctx)
	if err != nil {
		return nil, err
	}
	for _, cl := range clusters {
		if cl.Name == clusterName {
			return &cl, nil
		}
	}
	return nil, fmt.Errorf("EKS cluster %q not found in CloudScout — ensure the AWS connector is attached and discovery has run", clusterName)
}

func (c *Client) DeleteLogSource(ctx context.Context, name string) error {
	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/cs/api/%s/aws_logsource/%s", c.Project, name), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
