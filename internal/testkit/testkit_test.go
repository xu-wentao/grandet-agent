package testkit

import (
	"context"
	"errors"
	"testing"
)

func TestCallbackFakes(t *testing.T) {
	errWant := errors.New("expected")
	provider := FakeProvider[string, int]{ExecuteFunc: func(context.Context, string) (int, error) { return 1, errWant }}
	repository := FakeRepository[string, int]{GetFunc: func(context.Context, string) (int, error) { return 2, errWant }}
	validator := FakeValidator[string, bool]{ValidateFunc: func(context.Context, string) (bool, error) { return false, errWant }}

	if got, err := provider.Execute(context.Background(), "request"); got != 1 || !errors.Is(err, errWant) || len(provider.Requests) != 1 {
		t.Fatalf("provider: got=%d err=%v calls=%v", got, err, provider.Requests)
	}
	if got, err := repository.Get(context.Background(), "key"); got != 2 || !errors.Is(err, errWant) || len(repository.Keys) != 1 {
		t.Fatalf("repository: got=%d err=%v calls=%v", got, err, repository.Keys)
	}
	if got, err := validator.Validate(context.Background(), "input"); got || !errors.Is(err, errWant) || len(validator.Inputs) != 1 {
		t.Fatalf("validator: got=%t err=%v calls=%v", got, err, validator.Inputs)
	}
}
