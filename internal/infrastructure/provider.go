package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type ProviderConfigFile struct {
	Path string
}

func (f ProviderConfigFile) Load() ([]application.ProviderConfig, error) {
	contents, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, fmt.Errorf("read provider configuration: %w", err)
	}
	var document struct {
		SchemaVersion int `yaml:"schema_version"`
		Providers     map[string]struct {
			Type      string `yaml:"type"`
			BaseURL   string `yaml:"base_url"`
			APIKeyEnv string `yaml:"api_key_env"`
			Enabled   bool   `yaml:"enabled"`
		} `yaml:"providers"`
	}
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, fmt.Errorf("parse provider configuration: %w", err)
	}
	if document.SchemaVersion != 1 {
		return nil, fmt.Errorf("provider configuration schema_version must be 1")
	}
	if len(document.Providers) == 0 {
		return nil, fmt.Errorf("provider configuration has no providers")
	}
	configs := make([]application.ProviderConfig, 0, len(document.Providers))
	for name, provider := range document.Providers {
		config := application.ProviderConfig{Name: name, Type: provider.Type, BaseURL: provider.BaseURL, APIKeyEnv: provider.APIKeyEnv, Enabled: provider.Enabled}
		if err := validateProviderConfig(config); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].Name < configs[j].Name })
	return configs, nil
}

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateProviderConfig(config application.ProviderConfig) error {
	if config.Name == "" || config.Type == "" || config.BaseURL == "" {
		return fmt.Errorf("provider configuration requires name, type, and base_url")
	}
	baseURL, err := url.Parse(config.BaseURL)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.User != nil || baseURL.Fragment != "" || baseURL.RawQuery != "" {
		return fmt.Errorf("provider %q has invalid base_url", config.Name)
	}
	if config.APIKeyEnv != "" && !environmentName.MatchString(config.APIKeyEnv) {
		return fmt.Errorf("provider %q has invalid api_key_env", config.Name)
	}
	return nil
}

type Environment struct{}

func (Environment) Lookup(name string) (string, bool) { return os.LookupEnv(name) }

type OpenAICompatibleFactory struct {
	Client *http.Client
}

func (f OpenAICompatibleFactory) NewOpenAICompatible(config application.ProviderConfig, apiKey string) (domain.Provider, error) {
	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse provider base URL: %w", err)
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	return OpenAICompatibleProvider{baseURL: baseURL, apiKey: apiKey, client: client}, nil
}

type OpenAICompatibleProvider struct {
	baseURL *url.URL
	apiKey  string
	client  *http.Client
}

func (p OpenAICompatibleProvider) ListModels(ctx context.Context) ([]domain.ProviderModel, error) {
	response, err := p.do(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, malformedResponse(err, response.Header)
	}
	if len(payload.Data) == 0 || string(payload.Data) == "null" {
		return nil, malformedResponse(errors.New("models data is missing"), response.Header)
	}
	var data []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		return nil, malformedResponse(err, response.Header)
	}
	models := make([]domain.ProviderModel, len(data))
	for i, model := range data {
		if model.ID == "" {
			return nil, malformedResponse(errors.New("model id is missing"), response.Header)
		}
		models[i] = domain.ProviderModel{ID: model.ID}
	}
	return models, nil
}

func (p OpenAICompatibleProvider) Health(ctx context.Context) (domain.ProviderHealth, error) {
	response, err := p.do(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return domain.ProviderHealth{}, err
	}
	defer response.Body.Close()
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil || len(payload.Data) == 0 || string(payload.Data) == "null" {
		if err == nil {
			err = errors.New("models data is missing")
		}
		return domain.ProviderHealth{}, malformedResponse(err, response.Header)
	}
	return domain.ProviderHealth{RequestID: requestID(response.Header)}, nil
}

func (p OpenAICompatibleProvider) Execute(ctx context.Context, request domain.ProviderRequest) (domain.ProviderResponse, error) {
	messages := make([]openAIChatMessage, len(request.Messages))
	for i, message := range request.Messages {
		messages[i] = openAIChatMessage{Role: message.Role, Content: message.Content}
	}
	payload := openAIChatRequest{Model: request.Model, Messages: messages, MaxTokens: request.MaxOutputTokens}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.ProviderResponse{}, fmt.Errorf("marshal provider request: %w", err)
	}
	response, err := p.do(ctx, http.MethodPost, "/chat/completions", body)
	if err != nil {
		return domain.ProviderResponse{}, err
	}
	defer response.Body.Close()
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *openAIUsage `json:"usage"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return domain.ProviderResponse{}, malformedResponse(err, response.Header)
	}
	if len(result.Choices) == 0 {
		return domain.ProviderResponse{}, malformedResponse(errors.New("response has no choices"), response.Header)
	}
	var usage *domain.TokenUsage
	if result.Usage != nil {
		normalized := result.Usage.normalize()
		usage = &normalized
	}
	return domain.ProviderResponse{Text: result.Choices[0].Message.Content, Usage: usage, RequestID: requestID(response.Header)}, nil
}

type openAIChatRequest struct {
	Model     string              `json:"model"`
	Messages  []openAIChatMessage `json:"messages"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIUsage struct {
	PromptTokens  int `json:"prompt_tokens"`
	CachedDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokens int `json:"completion_tokens"`
	ReasoningDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

func (u openAIUsage) normalize() domain.TokenUsage {
	return domain.TokenUsage{InputTokens: u.PromptTokens, CachedTokens: u.CachedDetails.CachedTokens, OutputTokens: u.CompletionTokens, ReasoningTokens: u.ReasoningDetails.ReasoningTokens}
}

func (p OpenAICompatibleProvider) do(ctx context.Context, method, endpoint string, body []byte) (*http.Response, error) {
	target := *p.baseURL
	target.Path = strings.TrimSuffix(target.Path, "/") + endpoint
	request, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, &domain.ProviderError{Kind: domain.ProviderNetworkFailure, Detail: err.Error()}
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if p.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	response, err := p.client.Do(request)
	if err != nil {
		kind := domain.ProviderNetworkFailure
		if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
			kind = domain.ProviderTimeout
		}
		return nil, &domain.ProviderError{Kind: kind, Detail: err.Error()}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		detail := fmt.Sprintf("HTTP %d", response.StatusCode)
		if contents, readErr := io.ReadAll(io.LimitReader(response.Body, 4096)); readErr == nil && len(contents) > 0 {
			detail += ": " + strings.TrimSpace(string(contents))
		}
		response.Body.Close()
		return nil, &domain.ProviderError{Kind: classifyStatus(response.StatusCode, detail), Detail: redact(detail, p.apiKey), RequestID: requestID(response.Header)}
	}
	return response, nil
}

func classifyStatus(status int, detail string) domain.ProviderErrorKind {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return domain.ProviderAuthentication
	case http.StatusTooManyRequests:
		return domain.ProviderRateLimit
	case http.StatusNotFound:
		return domain.ProviderModelUnavailable
	case http.StatusBadRequest:
		lower := strings.ToLower(detail)
		if strings.Contains(lower, "context") || strings.Contains(lower, "maximum tokens") {
			return domain.ProviderContextWindowExceeded
		}
		if strings.Contains(lower, "model") {
			return domain.ProviderModelUnavailable
		}
	}
	return domain.ProviderNetworkFailure
}

func malformedResponse(err error, headers http.Header) *domain.ProviderError {
	return &domain.ProviderError{Kind: domain.ProviderMalformedResponse, Detail: err.Error(), RequestID: requestID(headers)}
}

func requestID(headers http.Header) string {
	for _, key := range []string{"x-request-id", "request-id", "x-openrouter-request-id"} {
		if value := headers.Get(key); value != "" {
			return value
		}
	}
	return ""
}

func redact(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[redacted]")
}
