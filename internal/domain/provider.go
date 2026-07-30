package domain

import (
	"context"
	"fmt"
)

type ProviderErrorKind string

const (
	ProviderTimeout               ProviderErrorKind = "timeout"
	ProviderRateLimit             ProviderErrorKind = "rate_limit"
	ProviderAuthentication        ProviderErrorKind = "authentication"
	ProviderModelUnavailable      ProviderErrorKind = "model_unavailable"
	ProviderContextWindowExceeded ProviderErrorKind = "context_window_exceeded"
	ProviderMalformedResponse     ProviderErrorKind = "malformed_response"
	ProviderNetworkFailure        ProviderErrorKind = "network_failure"
)

type ProviderError struct {
	Kind      ProviderErrorKind
	Detail    string
	RequestID string
}

func (e *ProviderError) Error() string {
	if e.RequestID == "" {
		return fmt.Sprintf("provider %s: %s", e.Kind, e.Detail)
	}
	return fmt.Sprintf("provider %s (request %s): %s", e.Kind, e.RequestID, e.Detail)
}

type ProviderModel struct {
	ID string
}

type ChatMessage struct {
	Role    string
	Content string
}

type ProviderRequest struct {
	Model           string
	Messages        []ChatMessage
	MaxOutputTokens int
}

type TokenUsage struct {
	InputTokens     int
	CachedTokens    int
	OutputTokens    int
	ReasoningTokens int
}

type ProviderResponse struct {
	Text      string
	Usage     *TokenUsage
	RequestID string
}

type ProviderHealth struct {
	RequestID string
}

// Provider is the application-facing provider port. Its values deliberately
// contain no SDK or HTTP transport types.
type Provider interface {
	ListModels(context.Context) ([]ProviderModel, error)
	Execute(context.Context, ProviderRequest) (ProviderResponse, error)
	Health(context.Context) (ProviderHealth, error)
}
