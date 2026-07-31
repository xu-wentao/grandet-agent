package testkit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFakes(t *testing.T) {
	if got := (Clock{Time: time.Unix(1, 0)}).Now(); !got.Equal(time.Unix(1, 0)) {
		t.Fatalf("clock = %v", got)
	}
	if got := (IDGenerator{ID: "id"}).New(); got != "id" {
		t.Fatalf("id = %q", got)
	}
	provider := Provider[string, string]{Call: func(_ context.Context, request string) (string, error) { return request, nil }}
	if got, err := provider.Invoke(context.Background(), "ok"); err != nil || got != "ok" {
		t.Fatalf("provider = %q, %v", got, err)
	}
	validator := Validator[string](func(string) error { return errors.New("invalid") })
	if validator.Validate("value") == nil {
		t.Fatal("validator error was lost")
	}
	repository := Repository[string, string]{Get: func(_ context.Context, id string) (string, error) { return id, nil }}
	if got, err := repository.Get(context.Background(), "stored"); err != nil || got != "stored" {
		t.Fatalf("repository = %q, %v", got, err)
	}
}
