package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorPreservesCauseAndCreatesSafeFailureEvent(t *testing.T) {
	inner := errors.New("provider response")
	cause := fmt.Errorf("outer: %w", inner)
	err := NewError(CodeProviderUnavailable, "provider request failed", true, Correlation{
		TrajectoryID:      "trj_1",
		StepID:            "step_1",
		ProviderRequestID: "req_1",
		PolicyVersion:     "stingy-v1",
	}, cause)

	if !errors.Is(err, inner) {
		t.Fatal("nested cause was not preserved")
	}
	resolved, ok := AsError(fmt.Errorf("wrapped: %w", err))
	if !ok || resolved != err {
		t.Fatalf("AsError() = %v, %v", resolved, ok)
	}
	event := NewFailureEvent(err)
	if event.ErrorCode != CodeProviderUnavailable || event.Correlation.ProviderRequestID != "req_1" || event.Message != "provider request failed" {
		t.Fatalf("failure event = %#v", event)
	}
}
