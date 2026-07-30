package infrastructure

import (
	"fmt"
	"strings"
	"testing"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		field  string
		secret string
	}{
		{name: "authorization header", input: "Authorization: Bearer sk_secret", secret: "sk_secret"},
		{name: "raw API key", input: "request failed with sk-proj-secret", secret: "sk-proj-secret"},
		{name: "api key header", input: "X-Api-Key: key-secret", secret: "key-secret"},
		{name: "credential URL", input: "https://user:password@example.com/v1", secret: "user:password"},
		{name: "query credential", input: "https://example.com?api_key=query-secret", secret: "query-secret"},
		{name: "configured field", input: `custom_token: configured-secret`, field: "custom_token", secret: "configured-secret"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := Redact(test.input, test.field)
			if strings.Contains(output, test.secret) || !strings.Contains(output, redacted) {
				t.Fatalf("Redact(%q) = %q", test.input, output)
			}
		})
	}
}

func TestNormalizeProviderFailureRedactsNestedCause(t *testing.T) {
	cause := fmt.Errorf("request: %w", fmt.Errorf("Authorization: Bearer sk_secret custom_token=field-secret"))
	err := NormalizeProviderFailure(ProviderFailure{
		Provider:   "openrouter",
		StatusCode: 429,
		RequestID:  "request-1",
		Cause:      cause,
	}, domain.Correlation{TrajectoryID: "trj-1"}, "custom_token")

	if err.Code != domain.CodeProviderRateLimited || !err.Retryable {
		t.Fatalf("provider failure = %#v", err)
	}
	if err.Correlation.ProviderRequestID != "request-1" {
		t.Fatalf("correlation = %#v", err.Correlation)
	}
	if strings.Contains(err.Provider.Message, "sk_secret") || strings.Contains(err.Provider.Message, "field-secret") {
		t.Fatalf("provider diagnostic leaked secret: %q", err.Provider.Message)
	}
	if err.Provider.StatusCode != 429 || err.Provider.RequestID != "request-1" {
		t.Fatalf("provider diagnostic = %#v", err.Provider)
	}
}

func TestProviderCode(t *testing.T) {
	tests := []struct {
		status    int
		wantCode  domain.ErrorCode
		retryable bool
	}{
		{status: 429, wantCode: domain.CodeProviderRateLimited, retryable: true},
		{status: 504, wantCode: domain.CodeProviderTimeout, retryable: true},
		{status: 503, wantCode: domain.CodeProviderUnavailable, retryable: true},
		{status: 401, wantCode: domain.CodeProviderRejected, retryable: false},
	}
	for _, test := range tests {
		code, retryable := providerCode(ProviderFailure{StatusCode: test.status})
		if code != test.wantCode || retryable != test.retryable {
			t.Errorf("providerCode(%d) = %q, %t", test.status, code, retryable)
		}
	}
}
