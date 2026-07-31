package application

import (
	"errors"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

// NormalizeError preserves typed failures and prevents raw operational errors
// from becoming CLI output.
func NormalizeError(err error) *domain.Error {
	var normalized *domain.Error
	if errors.As(err, &normalized) {
		return normalized
	}
	return domain.NewError(domain.CodeInternal, "unexpected failure; check configuration and try again", false, domain.Correlation{}, err)
}

func ValidationError(cause error) *domain.Error {
	return domain.NewError(domain.CodeValidation, "invalid command or option; run grandet help", false, domain.Correlation{}, cause)
}
