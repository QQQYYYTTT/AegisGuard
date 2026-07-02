package vkey

import (
	"net/http"
	"testing"
)

func TestExtractGatewayCredentialPrefersDedicatedHeader(t *testing.T) {
	headers := make(http.Header)
	headers.Set("X-Gateway-Key", "agk-test-001")
	headers.Set("Authorization", "Bearer sk-real-upstream")

	if got := ExtractGatewayCredential(headers); got != "agk-test-001" {
		t.Fatalf("gateway credential = %q, want agk-test-001", got)
	}
}

func TestExtractGatewayCredentialFallsBackToBearerGatewayKey(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer agk-test-001")

	if got := ExtractGatewayCredential(headers); got != "agk-test-001" {
		t.Fatalf("gateway credential = %q, want agk-test-001", got)
	}
}

func TestExtractGatewayCredentialIgnoresNonGatewayAuthorization(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer sk-real-upstream")

	if got := ExtractGatewayCredential(headers); got != "" {
		t.Fatalf("gateway credential = %q, want empty", got)
	}
}
