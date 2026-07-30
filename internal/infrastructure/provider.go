package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/xu-wentao/grandet-agent/internal/domain"
	"gopkg.in/yaml.v3"
)

type providerFile struct {
	Providers map[string]providerConfig `yaml:"providers"`
}

type providerConfig struct {
	Type      string `yaml:"type"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
	Enabled   bool   `yaml:"enabled"`
}

type modelsFile struct {
	Models            []modelConfig   `yaml:"models"`
	ExecutionProfiles []profileConfig `yaml:"execution_profiles"`
}

type modelConfig struct {
	ID           string `yaml:"id"`
	Provider     string `yaml:"provider"`
	UpstreamName string `yaml:"upstream_name"`
	Enabled      bool   `yaml:"enabled"`
}

type profileConfig struct {
	ID              string  `yaml:"id"`
	Model           string  `yaml:"model"`
	Enabled         bool    `yaml:"enabled"`
	MaxOutputTokens int     `yaml:"max_output_tokens"`
	Temperature     float64 `yaml:"temperature"`
}

// LoadProviderExecutor resolves an enabled fixed profile to its configured provider.
func LoadProviderExecutor(home, profileID string) (openAICompatibleExecutor, error) {
	var providers providerFile
	if err := readYAML(filepath.Join(home, "providers.yaml"), &providers); err != nil {
		return openAICompatibleExecutor{}, fmt.Errorf("read providers: %w", err)
	}
	var models modelsFile
	if err := readYAML(filepath.Join(home, "models.yaml"), &models); err != nil {
		return openAICompatibleExecutor{}, fmt.Errorf("read models: %w", err)
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
	provider, ok := providers.Providers[model.Provider]
	if !ok || !provider.Enabled || provider.Type != "openai_compatible" {
		return openAICompatibleExecutor{}, fmt.Errorf("enabled openai_compatible provider %q not found", model.Provider)
	}
	apiKey := os.Getenv(provider.APIKeyEnv)
	if provider.APIKeyEnv != "" && apiKey == "" {
		return openAICompatibleExecutor{}, fmt.Errorf("provider %q requires %s", model.Provider, provider.APIKeyEnv)
	}
	return openAICompatibleExecutor{client: http.DefaultClient, baseURL: strings.TrimRight(provider.BaseURL, "/"), apiKey: apiKey, model: model.UpstreamName, maxOutputTokens: profile.MaxOutputTokens, temperature: profile.Temperature}, nil
}

func readYAML(path string, value any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(contents, value)
}

type openAICompatibleExecutor struct {
	client          *http.Client
	baseURL         string
	apiKey          string
	model           string
	maxOutputTokens int
	temperature     float64
}

func (e openAICompatibleExecutor) Execute(ctx context.Context, prompt string) (domain.ProviderResult, error) {
	body, err := json.Marshal(map[string]any{"model": e.model, "messages": []map[string]string{{"role": "user", "content": prompt}}, "max_tokens": e.maxOutputTokens, "temperature": e.temperature})
	if err != nil {
		return domain.ProviderResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return domain.ProviderResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	response, err := e.client.Do(request)
	if err != nil {
		return domain.ProviderResult{}, err
	}
	defer response.Body.Close()
	result := domain.ProviderResult{}
	if requestID := response.Header.Get("x-request-id"); requestID != "" {
		result.ProviderRequestID = &requestID
	}
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return result, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("provider returned %s: %s", response.Status, strings.TrimSpace(string(contents)))
	}
	var completion struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens            *int     `json:"prompt_tokens"`
			CompletionTokens        *int     `json:"completion_tokens"`
			Cost                    *float64 `json:"cost"`
			CompletionTokensDetails struct {
				ReasoningTokens *int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(contents, &completion); err != nil {
		return result, fmt.Errorf("decode provider response: %w", err)
	}
	result.InputTokens = completion.Usage.PromptTokens
	result.OutputTokens = completion.Usage.CompletionTokens
	result.ReasoningTokens = completion.Usage.CompletionTokensDetails.ReasoningTokens
	result.ActualCostUSD = completion.Usage.Cost
	if result.ProviderRequestID == nil && completion.ID != "" {
		result.ProviderRequestID = &completion.ID
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message.Content == "" {
		return result, fmt.Errorf("provider returned no message content")
	}
	result.Output = completion.Choices[0].Message.Content
	return result, nil
}
