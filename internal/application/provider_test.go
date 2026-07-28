package application

import (
	"context"
	"strings"
	"testing"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type providerConfigs []ProviderConfig

func (c providerConfigs) Load() ([]ProviderConfig, error) { return c, nil }

type emptyCredentials struct{}

func (emptyCredentials) Lookup(string) (string, bool) { return "", false }

type unusedFactory struct{}

func (unusedFactory) NewOpenAICompatible(ProviderConfig, string) (domain.Provider, error) {
	return nil, nil
}

func TestProviderServiceRejectsDisabledAndMissingCredentials(t *testing.T) {
	for _, config := range []ProviderConfig{
		{Name: "disabled", Type: "openai_compatible", Enabled: false},
		{Name: "missing-key", Type: "openai_compatible", Enabled: true, APIKeyEnv: "TEST_PROVIDER_KEY"},
	} {
		t.Run(config.Name, func(t *testing.T) {
			_, err := NewProviderService(providerConfigs{config}, emptyCredentials{}, unusedFactory{}).Test(context.Background(), config.Name)
			if err == nil || (config.APIKeyEnv != "" && !strings.Contains(err.Error(), config.APIKeyEnv)) {
				t.Fatalf("expected local configuration error, got %v", err)
			}
		})
	}
}
