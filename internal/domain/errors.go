package domain

import "errors"

// ErrorCode is a stable machine-readable failure category.
type ErrorCode string

const (
	CodeConfiguration       ErrorCode = "configuration_error"
	CodeValidation          ErrorCode = "validation_error"
	CodeBudgetExceeded      ErrorCode = "budget_exceeded"
	CodeRoutingFailed       ErrorCode = "routing_failed"
	CodeProviderTimeout     ErrorCode = "provider_timeout"
	CodeProviderRateLimited ErrorCode = "provider_rate_limited"
	CodeProviderUnavailable ErrorCode = "provider_unavailable"
	CodeProviderRejected    ErrorCode = "provider_rejected"
	CodePersistenceFailure  ErrorCode = "persistence_failure"
	CodePolicyViolation     ErrorCode = "policy_violation"
	CodeInternal            ErrorCode = "internal_error"
)

// Correlation identifies the trajectory context in which a failure occurred.
type Correlation struct {
	TrajectoryID      string `json:"trajectory_id,omitempty"`
	StepID            string `json:"step_id,omitempty"`
	ProviderRequestID string `json:"provider_request_id,omitempty"`
	PolicyVersion     string `json:"policy_version,omitempty"`
}

// ProviderDiagnostic contains only provider metadata that is safe to present.
type ProviderDiagnostic struct {
	Provider   string `json:"provider,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
	Message    string `json:"message,omitempty"`
}

// Error separates a stable safe message from the original causal error.
type Error struct {
	Code        ErrorCode
	Message     string
	Retryable   bool
	Correlation Correlation
	Provider    *ProviderDiagnostic
	cause       error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.cause }

func NewError(code ErrorCode, message string, retryable bool, correlation Correlation, cause error) *Error {
	return &Error{Code: code, Message: message, Retryable: retryable, Correlation: correlation, cause: cause}
}

func NewProviderError(code ErrorCode, message string, retryable bool, correlation Correlation, diagnostic ProviderDiagnostic, cause error) *Error {
	return &Error{Code: code, Message: message, Retryable: retryable, Correlation: correlation, Provider: &diagnostic, cause: cause}
}

func AsError(err error) (*Error, bool) {
	var target *Error
	return target, errors.As(err, &target)
}

// FailureEvent is the safe error payload stored with a trajectory event.
type FailureEvent struct {
	Type        string      `json:"type"`
	ErrorCode   ErrorCode   `json:"error_code"`
	Message     string      `json:"message"`
	Retryable   bool        `json:"retryable"`
	Correlation Correlation `json:"correlation"`
}

func NewFailureEvent(err *Error) FailureEvent {
	return FailureEvent{
		Type:        "failure",
		ErrorCode:   err.Code,
		Message:     err.Message,
		Retryable:   err.Retryable,
		Correlation: err.Correlation,
	}
}
