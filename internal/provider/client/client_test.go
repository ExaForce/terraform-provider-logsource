package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer starts a mock HTTP server and returns a Client pointed at it.
// handlers maps "METHOD /path" to a handler func.
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	// Register whoami so fetchCSRF always succeeds unless overridden.
	if _, ok := handlers["GET /ae/rbac/system/whoami"]; !ok {
		mux.HandleFunc("/ae/rbac/system/whoami", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"csrf": "test-csrf-token"})
		})
	}

	for key, fn := range handlers {
		mux.HandleFunc(routePath(key), fn)
	}

	c := New(srv.URL, "", "test-id-token", "test-access-token", "test-session", "default")
	t.Cleanup(srv.Close)
	return srv, c
}

// routePath strips the "METHOD " prefix — ServeMux only takes paths.
func routePath(methodPath string) string {
	parts := strings.SplitN(methodPath, " ", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return methodPath
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// --- fetchCSRF ---

func TestFetchCSRF_Success(t *testing.T) {
	_, c := newTestServer(t, nil)
	tok, err := c.fetchCSRF(context.Background())
	if err != nil {
		t.Fatalf("fetchCSRF: %v", err)
	}
	if tok != "test-csrf-token" {
		t.Errorf("got %q, want %q", tok, "test-csrf-token")
	}
}

func TestFetchCSRF_Cached(t *testing.T) {
	calls := 0
	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ae/rbac/system/whoami": func(w http.ResponseWriter, r *http.Request) {
			calls++
			writeJSON(w, http.StatusOK, map[string]string{"csrf": "cached-token"})
		},
	})

	c.fetchCSRF(context.Background())
	c.fetchCSRF(context.Background())

	if calls != 1 {
		t.Errorf("whoami called %d times, want 1 (should be cached)", calls)
	}
}

func TestFetchCSRF_ServerError(t *testing.T) {
	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ae/rbac/system/whoami": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})

	_, err := c.fetchCSRF(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- CreateLogSource ---

func TestCreateLogSource_Success(t *testing.T) {
	want := LogSource{
		Spec:     LogSourceSpec{LogSourceType: "eks", EksArn: "arn:aws:eks:us-east-1:123:cluster/test"},
		Metadata: LogSourceMetadata{Name: "eks-123-us-east-1-test", Project: "default"},
	}

	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"POST /cs/api/default/aws_logsources": func(w http.ResponseWriter, r *http.Request) {
			// Verify CSRF query param is present.
			if r.URL.Query().Get("csrf") == "" {
				t.Error("expected csrf query param, got none")
			}
			// Verify request body contains name.
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] == nil {
				t.Error("request body missing name field")
			}
			writeJSON(w, http.StatusOK, want)
		},
	})

	got, err := c.CreateLogSource(context.Background(), want.Metadata.Name, want.Spec)
	if err != nil {
		t.Fatalf("CreateLogSource: %v", err)
	}
	if got.Name() != want.Metadata.Name {
		t.Errorf("name: got %q, want %q", got.Name(), want.Metadata.Name)
	}
	if got.Spec.EksArn != want.Spec.EksArn {
		t.Errorf("eks_arn: got %q, want %q", got.Spec.EksArn, want.Spec.EksArn)
	}
}

func TestCreateLogSource_APIError(t *testing.T) {
	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"POST /cs/api/default/aws_logsources": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"message": "EKS Cluster arn:... not found",
			})
		},
	})

	_, err := c.CreateLogSource(context.Background(), "test", LogSourceSpec{LogSourceType: "eks"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got: %v", err)
	}
}

// --- GetLogSource ---

func TestGetLogSource_Found(t *testing.T) {
	want := LogSource{
		Spec:     LogSourceSpec{LogSourceType: "eks", EksArn: "arn:aws:eks:us-east-1:123:cluster/test"},
		Metadata: LogSourceMetadata{Name: "test-source", Project: "default"},
	}

	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"GET /cs/api/default/aws_logsource/test-source": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, want)
		},
	})

	got, status, err := c.GetLogSource(context.Background(), "test-source")
	if err != nil {
		t.Fatalf("GetLogSource: %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("status: got %d, want 200", status)
	}
	if got.Name() != "test-source" {
		t.Errorf("name: got %q, want %q", got.Name(), "test-source")
	}
}

func TestGetLogSource_NotFound(t *testing.T) {
	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"GET /cs/api/default/aws_logsource/missing": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
		},
	})

	_, status, err := c.GetLogSource(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", status)
	}
}

// --- ListLogSources ---

func TestListLogSources_Success(t *testing.T) {
	sources := []LogSource{
		{Spec: LogSourceSpec{LogSourceType: "eks"}, Metadata: LogSourceMetadata{Name: "source-a"}},
		{Spec: LogSourceSpec{LogSourceType: "cloudtrail"}, Metadata: LogSourceMetadata{Name: "source-b"}},
	}

	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"GET /cs/api/default/aws_logsources": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, sources)
		},
	})

	got, err := c.ListLogSources(context.Background())
	if err != nil {
		t.Fatalf("ListLogSources: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len: got %d, want 2", len(got))
	}
	if got[0].Name() != "source-a" {
		t.Errorf("got[0].Name: %q, want %q", got[0].Name(), "source-a")
	}
}

func TestListLogSources_Empty(t *testing.T) {
	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"GET /cs/api/default/aws_logsources": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, []LogSource{})
		},
	})

	got, err := c.ListLogSources(context.Background())
	if err != nil {
		t.Fatalf("ListLogSources: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d items", len(got))
	}
}

// --- DeleteLogSource ---

func TestDeleteLogSource_Success(t *testing.T) {
	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /cs/api/default/aws_logsource/to-delete": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})

	if err := c.DeleteLogSource(context.Background(), "to-delete"); err != nil {
		t.Fatalf("DeleteLogSource: %v", err)
	}
}

func TestDeleteLogSource_NotFound_IsNoop(t *testing.T) {
	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /cs/api/default/aws_logsource/gone": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
		},
	})

	// 404 on delete should be treated as a no-op (idempotent destroy).
	if err := c.DeleteLogSource(context.Background(), "gone"); err != nil {
		t.Fatalf("expected no error on 404 delete, got: %v", err)
	}
}

func TestDeleteLogSource_APIToken_SkipsCSRF(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// No whoami handler — API token auth must not call it.
	mux.HandleFunc("/cs/api/default/aws_logsource/src", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-EXF-API-TOKEN") == "" {
			t.Error("expected X-EXF-API-TOKEN header")
		}
		if r.URL.Query().Get("csrf") != "" {
			t.Error("API token auth must not append csrf query param")
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(srv.URL, "my-api-token", "", "", "", "default")
	if err := c.DeleteLogSource(context.Background(), "src"); err != nil {
		t.Fatalf("DeleteLogSource with API token: %v", err)
	}
}

// --- LookupEKSClusterByName ---

func TestLookupEKSClusterByName_Found(t *testing.T) {
	clusters := []map[string]any{
		{"spec": map[string]any{"name": "other-eks", "arn": "arn:aws:eks:us-east-1:111:cluster/other-eks"}},
		{"spec": map[string]any{"name": "target-eks", "arn": "arn:aws:eks:us-east-1:222:cluster/target-eks"}},
	}

	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"GET /cs/api/default/aws_eks_clusters": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, clusters)
		},
	})

	got, err := c.LookupEKSClusterByName(context.Background(), "target-eks")
	if err != nil {
		t.Fatalf("LookupEKSClusterByName: %v", err)
	}
	if got.ARN != "arn:aws:eks:us-east-1:222:cluster/target-eks" {
		t.Errorf("ARN: got %q, want %q", got.ARN, "arn:aws:eks:us-east-1:222:cluster/target-eks")
	}
}

func TestLookupEKSClusterByName_NotFound(t *testing.T) {
	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"GET /cs/api/default/aws_eks_clusters": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, []any{})
		},
	})

	_, err := c.LookupEKSClusterByName(context.Background(), "missing-eks")
	if err == nil {
		t.Fatal("expected error for missing cluster, got nil")
	}
	if !strings.Contains(err.Error(), "not found in CloudScout") {
		t.Errorf("expected 'not found in CloudScout' in error, got: %v", err)
	}
}

// --- invalidateCSRF ---

func TestInvalidateCSRF_RefetchesOnNextCall(t *testing.T) {
	calls := 0
	_, c := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ae/rbac/system/whoami": func(w http.ResponseWriter, r *http.Request) {
			calls++
			writeJSON(w, http.StatusOK, map[string]string{"csrf": "token"})
		},
	})

	c.fetchCSRF(context.Background())
	c.invalidateCSRF()
	c.fetchCSRF(context.Background())

	if calls != 2 {
		t.Errorf("whoami called %d times after invalidate, want 2", calls)
	}
}
