package infrastructure

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

const redacted = "[REDACTED]"

var (
	credentialURLPattern = regexp.MustCompile(`([[:alpha:]][[:alnum:]+.-]*://)[^/@[:space:]]+@`)
	headerPattern        = regexp.MustCompile(`(?im)\b(authorization|proxy-authorization|x-api-key|api-key|api_key)\s*[:=]\s*[^\r\n]*`)
	queryPattern         = regexp.MustCompile(`(?i)([?&](?:api[_-]?key|access[_-]?token|token|password|secret)=)[^&#[:space:]]+`)
	secretTokenPattern   = regexp.MustCompile(`\b(?:sk|rk|pk)[_-][[:alnum:]_-]+`)
)

// Redact removes credentials from diagnostics before they reach users or events.
func Redact(value string, sensitiveFields ...string) string {
	value = credentialURLPattern.ReplaceAllString(value, "${1}"+redacted+"@")
	value = headerPattern.ReplaceAllStringFunc(value, func(match string) string {
		separator := strings.IndexAny(match, ":=")
		return match[:separator] + string(match[separator]) + " " + redacted
	})
	value = queryPattern.ReplaceAllString(value, "${1}"+redacted)
	value = secretTokenPattern.ReplaceAllString(value, redacted)
	for _, field := range sensitiveFields {
		if field == "" {
			continue
		}
		pattern := regexp.MustCompile(`(?i)(["']?` + regexp.QuoteMeta(field) + `["']?\s*[:=]\s*["']?)[^"'[:space:],}]+`)
		value = pattern.ReplaceAllString(value, "${1}"+redacted)
	}
	return value
}

// ProviderFailure is the provider adapter input for stable error mapping.
type ProviderFailure struct {
	Provider   string
	StatusCode int
	RequestID  string
	Message    string
	Cause      error
}

// NormalizeProviderFailure preserves the cause while exposing only redacted diagnostics.
func NormalizeProviderFailure(failure ProviderFailure, correlation domain.Correlation, sensitiveFields ...string) *domain.Error {
	if correlation.ProviderRequestID == "" {
		correlation.ProviderRequestID = failure.RequestID
	}
	diagnostic := domain.ProviderDiagnostic{
		Provider:   failure.Provider,
		StatusCode: failure.StatusCode,
		RequestID:  failure.RequestID,
		Message:    Redact(failure.Message, sensitiveFields...),
	}
	if diagnostic.Message == "" && failure.Cause != nil {
		diagnostic.Message = Redact(failure.Cause.Error(), sensitiveFields...)
	}
	code, retryable := providerCode(failure)
	return domain.NewProviderError(code, "provider request failed; check provider availability or configuration", retryable, correlation, diagnostic, failure.Cause)
}

func providerCode(failure ProviderFailure) (domain.ErrorCode, bool) {
	switch failure.StatusCode {
	case 429:
		return domain.CodeProviderRateLimited, true
	case 408, 504:
		return domain.CodeProviderTimeout, true
	}
	if failure.StatusCode >= 500 {
		return domain.CodeProviderUnavailable, true
	}
	if failure.StatusCode >= 400 {
		return domain.CodeProviderRejected, false
	}
	if errors.Is(failure.Cause, context.DeadlineExceeded) {
		return domain.CodeProviderTimeout, true
	}
	var networkError net.Error
	if errors.As(failure.Cause, &networkError) && networkError.Timeout() {
		return domain.CodeProviderTimeout, true
	}
	var providerError *domain.ProviderError
	if errors.As(failure.Cause, &providerError) {
		switch providerError.Kind {
		case domain.ProviderTimeout:
			return domain.CodeProviderTimeout, true
		case domain.ProviderRateLimit:
			return domain.CodeProviderRateLimited, true
		case domain.ProviderAuthentication, domain.ProviderModelUnavailable, domain.ProviderContextWindowExceeded:
			return domain.CodeProviderRejected, false
		}
	}
	return domain.CodeProviderUnavailable, true
}
