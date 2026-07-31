package application

import (
	"context"
	"fmt"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type ProviderConfig struct {
	Name      string
	Type      string
	BaseURL   string
	APIKeyEnv string
	Enabled   bool
}

type ProviderConfigLoader interface {
	Load() ([]ProviderConfig, error)
}

type CredentialResolver interface {
	Lookup(name string) (string, bool)
}

type ProviderFactory interface {
	NewOpenAICompatible(ProviderConfig, string) (domain.Provider, error)
}

type ProviderService struct {
	configs     ProviderConfigLoader
	credentials CredentialResolver
	factory     ProviderFactory
}

func NewProviderService(configs ProviderConfigLoader, credentials CredentialResolver, factory ProviderFactory) ProviderService {
	return ProviderService{configs: configs, credentials: credentials, factory: factory}
}

func (s ProviderService) List() ([]ProviderConfig, error) {
	return s.configs.Load()
}

func (s ProviderService) Test(ctx context.Context, name string) (domain.ProviderHealth, error) {
	provider, err := s.provider(name)
	if err != nil {
		return domain.ProviderHealth{}, err
	}
	return provider.Health(ctx)
}

func (s ProviderService) ListModels(ctx context.Context, name string) ([]domain.ProviderModel, error) {
	provider, err := s.provider(name)
	if err != nil {
		return nil, err
	}
	return provider.ListModels(ctx)
}

func (s ProviderService) provider(name string) (domain.Provider, error) {
	configs, err := s.configs.Load()
	if err != nil {
		return nil, err
	}
	for _, config := range configs {
		if config.Name != name {
			continue
		}
		if !config.Enabled {
			return nil, fmt.Errorf("provider %q is disabled", name)
		}
		if config.Type != "openai_compatible" {
			return nil, fmt.Errorf("provider %q has unsupported type %q", name, config.Type)
		}
		apiKey := ""
		if config.APIKeyEnv != "" {
			var ok bool
			apiKey, ok = s.credentials.Lookup(config.APIKeyEnv)
			if !ok || apiKey == "" {
				return nil, fmt.Errorf("provider %q requires environment variable %s", name, config.APIKeyEnv)
			}
		}
		provider, err := s.factory.NewOpenAICompatible(config, apiKey)
		if err != nil {
			return nil, err
		}
		return provider, nil
	}
	return nil, fmt.Errorf("provider %q is not configured", name)
}
