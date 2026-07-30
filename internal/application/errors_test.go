package application

import (
	"errors"
	"testing"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

func TestNormalizeErrorUsesInternalCodeForUntypedFailures(t *testing.T) {
	if got := NormalizeError(errors.New("operation failed")); got.Code != domain.CodeInternal {
		t.Fatalf("code = %q, want %q", got.Code, domain.CodeInternal)
	}
}
