package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ExaForce/terraform-provider-logsource/internal/provider/client"
)

func TestDeriveLogSourceName(t *testing.T) {
	tests := []struct {
		name string
		spec client.LogSourceSpec
		want string
	}{
		{
			name: "standard EKS ARN",
			spec: client.LogSourceSpec{
				LogSourceType: "eks",
				EksArn:        "arn:aws:eks:us-east-1:119473394764:cluster/exaforce-eks",
			},
			want: "eks-119473394764-us-east-1-exaforce-eks",
		},
		{
			name: "EKS in us-west-2",
			spec: client.LogSourceSpec{
				LogSourceType: "eks",
				EksArn:        "arn:aws:eks:us-west-2:498202685299:cluster/halozyme-eks",
			},
			want: "eks-498202685299-us-west-2-halozyme-eks",
		},
		{
			name: "EKS cluster name with multiple hyphens",
			spec: client.LogSourceSpec{
				LogSourceType: "eks",
				EksArn:        "arn:aws:eks:ap-northeast-1:363661057649:cluster/openhouse-prod-eks",
			},
			want: "eks-363661057649-ap-northeast-1-openhouse-prod-eks",
		},
		{
			name: "unknown type falls back to type-logsource",
			spec: client.LogSourceSpec{
				LogSourceType: "cloudtrail",
			},
			want: "cloudtrail-logsource",
		},
		{
			name: "EKS ARN missing cluster segment falls back",
			spec: client.LogSourceSpec{
				LogSourceType: "eks",
				EksArn:        "not-an-arn",
			},
			want: "eks-logsource",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveLogSourceName(tc.spec)
			if got != tc.want {
				t.Errorf("deriveLogSourceName(%+v) = %q, want %q", tc.spec, got, tc.want)
			}
		})
	}
}

func TestEKSErrorDetail_AuditLogging(t *testing.T) {
	apiErr := "API error 400: {\"message\":\"EKS Cluster arn:aws:eks:us-east-1:123:cluster/my-eks is not configured for audit logging\"}"
	eksArn := "arn:aws:eks:us-east-1:123:cluster/my-eks"
	got := eksErrorDetail(apiErr, eksArn)

	if !strings.Contains(got, `enabled_cluster_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]`) {
		t.Errorf("expected audit logging terraform snippet in error, got:\n%s", got)
	}
	if !strings.Contains(got, `"aws_eks_cluster" "my-eks"`) {
		t.Errorf("expected cluster name in terraform snippet, got:\n%s", got)
	}
}

func TestEKSErrorDetail_SubscriptionFilter(t *testing.T) {
	destArn := "arn:aws:logs:us-east-2:913524906762:destination:eks-cloudwatch-logs-destination-default-us-east-2"
	apiErr := fmt.Sprintf("API error 400: {\"message\":\"EKS Cluster arn:aws:eks:us-east-2:934:cluster/demo-eks CloudWatch LogGroup /aws/eks/demo-eks/cluster doesn't have configured subscription filter %s\"}", destArn)
	eksArn := "arn:aws:eks:us-east-2:934:cluster/demo-eks"
	got := eksErrorDetail(apiErr, eksArn)

	if !strings.Contains(got, "aws_cloudwatch_log_subscription_filter") {
		t.Errorf("expected terraform resource in error, got:\n%s", got)
	}
	if !strings.Contains(got, destArn) {
		t.Errorf("expected destination ARN in terraform snippet, got:\n%s", got)
	}
	if !strings.Contains(got, `/aws/eks/demo-eks/cluster`) {
		t.Errorf("expected log group name in terraform snippet, got:\n%s", got)
	}
	if !strings.Contains(got, "exaforce-demo-eks-filter-us-east-2") {
		t.Errorf("expected filter name in terraform snippet, got:\n%s", got)
	}
}

func TestEKSErrorDetail_Unknown(t *testing.T) {
	apiErr := "API error 500: internal server error"
	got := eksErrorDetail(apiErr, "arn:aws:eks:us-east-1:123:cluster/test")
	if got != apiErr {
		t.Errorf("unknown error should pass through unchanged, got:\n%s", got)
	}
}

func TestDeriveLogSourceName_TruncatesTo63Chars(t *testing.T) {
	spec := client.LogSourceSpec{
		LogSourceType: "eks",
		// cluster name that would produce > 63 chars total
		EksArn: "arn:aws:eks:us-east-1:123456789012:cluster/very-long-cluster-name-that-exceeds-limit",
	}
	got := deriveLogSourceName(spec)
	if len(got) > 63 {
		t.Errorf("name length %d exceeds 63: %q", len(got), got)
	}
	// Must not end with a hyphen (pattern: ^[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?$)
	if len(got) > 0 && got[len(got)-1] == '-' {
		t.Errorf("name ends with hyphen: %q", got)
	}
}
