package infrastructure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/domain"
)

func testOpenAIProvider(t *testing.T, handler http.HandlerFunc) domain.Provider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	provider, err := (OpenAICompatibleFactory{}).NewOpenAICompatible(application.ProviderConfig{BaseURL: server.URL + "/v1"}, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func TestOpenAICompatibleProviderExecuteExtractsUsage(t *testing.T) {
	provider := testOpenAIProvider(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" || request.Header.Get("Authorization") != "Bearer test-secret" {
			t.Fatalf("unexpected request: %s auth=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		var payload struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "test" || len(payload.Messages) != 1 || payload.Messages[0].Role != "user" || payload.Messages[0].Content != "hi" {
			t.Fatalf("unexpected OpenAI request payload: %#v", payload)
		}
		writer.Header().Set("X-Request-ID", "req_123")
		writer.Write([]byte(`{"choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":11,"prompt_tokens_details":{"cached_tokens":3},"completion_tokens":7,"completion_tokens_details":{"reasoning_tokens":2}}}`))
	})

	response, err := provider.Execute(context.Background(), domain.ProviderRequest{Model: "test", Messages: []domain.ChatMessage{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "hello" || response.RequestID != "req_123" || response.Usage == nil || *response.Usage != (domain.TokenUsage{InputTokens: 11, CachedTokens: 3, OutputTokens: 7, ReasoningTokens: 2}) {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestOpenAICompatibleProviderExecuteLeavesOmittedUsageUnknown(t *testing.T) {
	provider := testOpenAIProvider(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	})

	response, err := provider.Execute(context.Background(), domain.ProviderRequest{Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Usage != nil {
		t.Fatalf("usage = %#v, want nil for omitted provider usage", response.Usage)
	}
}

func TestOpenAICompatibleProviderNormalizesFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		context func() (context.Context, context.CancelFunc)
		want    domain.ErrorCode
	}{
		{
			name:    "timeout",
			handler: func(writer http.ResponseWriter, request *http.Request) { time.Sleep(50 * time.Millisecond) },
			context: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Millisecond)
			},
			want: domain.CodeProviderTimeout,
		},
		{
			name: "rate limit",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				http.Error(writer, "slow down", http.StatusTooManyRequests)
			},
			context: func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			want:    domain.CodeProviderRateLimited,
		},
		{
			name: "authentication",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				http.Error(writer, "bad key", http.StatusUnauthorized)
			},
			context: func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			want:    domain.CodeProviderRejected,
		},
		{
			name:    "malformed response",
			handler: func(writer http.ResponseWriter, request *http.Request) { writer.Write([]byte(`{"choices":`)) },
			context: func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			want:    domain.CodeProviderUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := testOpenAIProvider(t, test.handler)
			ctx, cancel := test.context()
			defer cancel()
			_, err := provider.Execute(ctx, domain.ProviderRequest{Model: "test"})
			providerError, ok := domain.AsError(err)
			if !ok || providerError.Code != test.want {
				t.Fatalf("error = %#v, want %s", err, test.want)
			}
		})
	}
}

func TestOpenAICompatibleProviderFailureRedactsBasicCredentials(t *testing.T) {
	provider := testOpenAIProvider(t, func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "Authorization: Basic dXNlcjpwYXNz", http.StatusTooManyRequests)
	})

	_, err := provider.Execute(context.Background(), domain.ProviderRequest{Model: "test"})
	normalized, ok := domain.AsError(err)
	if !ok || normalized.Code != domain.CodeProviderRateLimited {
		t.Fatalf("error = %#v", err)
	}
	if strings.Contains(normalized.Provider.Message, "dXNlcjpwYXNz") {
		t.Fatalf("provider diagnostic leaked credentials: %q", normalized.Provider.Message)
	}
}

func TestOpenAICompatibleProviderFailureRedactsConfiguredAPIKey(t *testing.T) {
	const apiKey = "arbitrary-format-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "invalid key "+apiKey, http.StatusUnauthorized)
	}))
	defer server.Close()
	provider, err := (OpenAICompatibleFactory{}).NewOpenAICompatible(application.ProviderConfig{BaseURL: server.URL + "/v1"}, apiKey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.Execute(context.Background(), domain.ProviderRequest{Model: "test"})
	normalized, ok := domain.AsError(err)
	if !ok || normalized.Provider == nil || strings.Contains(normalized.Provider.Message, apiKey) {
		t.Fatalf("provider diagnostic leaked configured API key: %#v", err)
	}
}

func TestOpenAICompatibleProviderHealthDoesNotGenerate(t *testing.T) {
	provider := testOpenAIProvider(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/models" {
			t.Fatalf("health made a generation request: %s %s", request.Method, request.URL.Path)
		}
		writer.Header().Set("X-Request-ID", "health_123")
		writer.Write([]byte(`{"data":[]}`))
	})
	health, err := provider.Health(context.Background())
	if err != nil || health.RequestID != "health_123" {
		t.Fatalf("health = %#v, %v", health, err)
	}
}

func TestProviderConfigFileLoadsAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.yaml")
	if err := os.WriteFile(path, []byte("schema_version: 1\nproviders:\n  local:\n    type: openai_compatible\n    base_url: http://localhost:4000/v1\n    enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configs, err := (ProviderConfigFile{Path: path}).Load()
	if err != nil || len(configs) != 1 || configs[0].Name != "local" {
		t.Fatalf("configs = %#v, %v", configs, err)
	}
	if err := os.WriteFile(path, []byte("schema_version: 2\nproviders: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (ProviderConfigFile{Path: path}).Load(); err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("invalid schema error = %v", err)
	}
}
