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
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

type modelsFile struct {
	SchemaVersion     int             `yaml:"schema_version"`
	Models            []modelConfig   `yaml:"models"`
	ExecutionProfiles []profileConfig `yaml:"execution_profiles"`
}

type modelConfig struct {
	ID             string   `yaml:"id"`
	Provider       string   `yaml:"provider"`
	UpstreamName   string   `yaml:"upstream_name"`
	Enabled        bool     `yaml:"enabled"`
	IsFree         bool     `yaml:"is_free"`
	LifecycleState string   `yaml:"lifecycle_state"`
	ContextWindow  *int     `yaml:"context_window"`
	Capabilities   []string `yaml:"capabilities"`
}

type profileConfig struct {
	ID        string `yaml:"id"`
	Model     string `yaml:"model"`
	Enabled   bool   `yaml:"enabled"`
	Reasoning struct {
		Mode   string `yaml:"mode"`
		Effort string `yaml:"effort"`
	} `yaml:"reasoning"`
	MaxOutputTokens int     `yaml:"max_output_tokens"`
	Temperature     float64 `yaml:"temperature"`
	ToolCalling     bool    `yaml:"tool_calling"`
	JSONOutput      bool    `yaml:"json_output"`
	Vision          bool    `yaml:"vision"`
	RetryPolicy     string  `yaml:"retry_policy"`
	QualityTier     string  `yaml:"quality_tier"`
}

// LoadProviderExecutor resolves an enabled fixed profile through the shared provider adapter.
func LoadProviderExecutor(home, profileID string) (openAICompatibleExecutor, error) {
	configs, err := (ProviderConfigFile{Path: filepath.Join(home, "providers.yaml")}).Load()
	if err != nil {
		return openAICompatibleExecutor{}, err
	}
	contents, err := os.ReadFile(filepath.Join(home, "models.yaml"))
	if err != nil {
		return openAICompatibleExecutor{}, fmt.Errorf("read models: %w", err)
	}
	var models modelsFile
	if err := yaml.Unmarshal(contents, &models); err != nil {
		return openAICompatibleExecutor{}, fmt.Errorf("parse models: %w", err)
	}
	var profile profileConfig
	for _, candidate := range models.ExecutionProfiles {
		if candidate.ID == profileID {
			profile = candidate
			break
		}
	}
	if profile.ID == "" || !profile.Enabled {
		return openAICompatibleExecutor{}, fmt.Errorf("enabled execution profile %q not found", profileID)
	}
	var model modelConfig
	for _, candidate := range models.Models {
		if candidate.ID == profile.Model {
			model = candidate
			break
		}
	}
	if model.ID == "" || !model.Enabled {
		return openAICompatibleExecutor{}, fmt.Errorf("enabled model %q not found", profile.Model)
	}
	for _, config := range configs {
		if config.Name != model.Provider {
			continue
		}
		if !config.Enabled || config.Type != "openai_compatible" {
			break
		}
		apiKey := ""
		if config.APIKeyEnv != "" {
			var ok bool
			apiKey, ok = Environment{}.Lookup(config.APIKeyEnv)
			if !ok || apiKey == "" {
				return openAICompatibleExecutor{}, fmt.Errorf("provider %q requires %s", config.Name, config.APIKeyEnv)
			}
		}
		provider, err := (OpenAICompatibleFactory{}).NewOpenAICompatible(config, apiKey)
		if err != nil {
			return openAICompatibleExecutor{}, err
		}
		return openAICompatibleExecutor{provider: provider, model: model.UpstreamName, maxOutputTokens: profile.MaxOutputTokens, temperature: profile.Temperature}, nil
	}
	return openAICompatibleExecutor{}, fmt.Errorf("enabled openai_compatible provider %q not found", model.Provider)
}

type openAICompatibleExecutor struct {
	provider        domain.Provider
	model           string
	maxOutputTokens int
	temperature     float64
}

func (e openAICompatibleExecutor) Execute(ctx context.Context, prompt string) (domain.ProviderResult, error) {
	response, err := e.provider.Execute(ctx, domain.ProviderRequest{Model: e.model, Messages: []domain.ChatMessage{{Role: "user", Content: prompt}}, MaxOutputTokens: e.maxOutputTokens, Temperature: e.temperature})
	result := domain.ProviderResult{Output: response.Text, ActualCostUSD: response.ActualCostUSD}
	if response.RequestID != "" {
		result.ProviderRequestID = &response.RequestID
	}
	if response.Usage != nil {
		result.InputTokens = response.Usage.InputTokens
		result.OutputTokens = response.Usage.OutputTokens
		result.ReasoningTokens = response.Usage.ReasoningTokens
	}
	if err != nil {
		var providerError *domain.ProviderError
		if errors.As(err, &providerError) && providerError.RequestID != "" {
			result.ProviderRequestID = &providerError.RequestID
		}
	}
	return result, err
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
		ID            string `json:"id"`
		ContextLength *int   `json:"context_length"`
		Architecture  struct {
			InputModalities  []string `json:"input_modalities"`
			OutputModalities []string `json:"output_modalities"`
		} `json:"architecture"`
		SupportedParameters []string `json:"supported_parameters"`
		Pricing             struct {
			Prompt            *string `json:"prompt"`
			Completion        *string `json:"completion"`
			InternalReasoning *string `json:"internal_reasoning"`
			CacheRead         *string `json:"cache_read"`
		} `json:"pricing"`
	}
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		return nil, malformedResponse(err, response.Header)
	}
	models := make([]domain.ProviderModel, len(data))
	for i, model := range data {
		if model.ID == "" {
			return nil, malformedResponse(errors.New("model id is missing"), response.Header)
		}
		capabilities := domain.ModelCapabilities{ToolCalling: contains(model.SupportedParameters, "tools"), JSONOutput: contains(model.SupportedParameters, "response_format"), Vision: contains(model.Architecture.InputModalities, "image")}
		metadata, err := json.Marshal(struct {
			InputModalities  []string `json:"input_modalities,omitempty"`
			OutputModalities []string `json:"output_modalities,omitempty"`
		}{model.Architecture.InputModalities, model.Architecture.OutputModalities})
		if err != nil {
			return nil, err
		}
		price := openRouterPrice(model.Pricing.Prompt, model.Pricing.Completion, model.Pricing.InternalReasoning, model.Pricing.CacheRead)
		models[i] = domain.ProviderModel{ID: model.ID, ContextWindow: model.ContextLength, Capabilities: capabilities, IsFree: isFree(price), Price: price, SourceMetadata: string(metadata)}
	}
	return models, nil
}

func openRouterPrice(prompt, completion, reasoning, cacheRead *string) *domain.ModelPrice {
	input, inputOK := perMillion(prompt)
	output, outputOK := perMillion(completion)
	if !inputOK && !outputOK {
		return nil
	}
	price := &domain.ModelPrice{InputPerMillion: input, OutputPerMillion: output, Source: "provider_sync"}
	price.ReasoningPerMillion, _ = perMillion(reasoning)
	price.CachedInputPerMillion, _ = perMillion(cacheRead)
	return price
}

func perMillion(value *string) (*float64, bool) {
	if value == nil || *value == "" {
		return nil, false
	}
	parsed, err := strconv.ParseFloat(*value, 64)
	if err != nil {
		return nil, false
	}
	parsed *= 1_000_000
	return &parsed, true
}

func isFree(price *domain.ModelPrice) bool {
	return price != nil && price.InputPerMillion != nil && price.OutputPerMillion != nil && *price.InputPerMillion == 0 && *price.OutputPerMillion == 0
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
	payload := openAIChatRequest{Model: request.Model, Messages: messages, MaxTokens: request.MaxOutputTokens, Temperature: request.Temperature}
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
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return domain.ProviderResponse{}, malformedResponse(errors.New("response has no choices"), response.Header)
	}
	var usage *domain.TokenUsage
	if result.Usage != nil {
		normalized := result.Usage.normalize()
		usage = &normalized
	}
	var cost *float64
	if result.Usage != nil {
		cost = result.Usage.Cost
	}
	return domain.ProviderResponse{Text: result.Choices[0].Message.Content, Usage: usage, RequestID: requestID(response.Header), ActualCostUSD: cost}, nil
}

type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIUsage struct {
	PromptTokens  *int `json:"prompt_tokens"`
	CachedDetails *struct {
		CachedTokens *int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokens *int     `json:"completion_tokens"`
	Cost             *float64 `json:"cost"`
	ReasoningDetails *struct {
		ReasoningTokens *int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

func (u openAIUsage) normalize() domain.TokenUsage {
	usage := domain.TokenUsage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens}
	if u.CachedDetails != nil {
		usage.CachedTokens = u.CachedDetails.CachedTokens
	}
	if u.ReasoningDetails != nil {
		usage.ReasoningTokens = u.ReasoningDetails.ReasoningTokens
	}
	return usage
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
