package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/infrastructure"
)

func runProvider(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: grandet provider <list|test> [provider] [--home path]")
	}
	switch args[0] {
	case "list":
		return runProviderList(args[1:])
	case "test":
		return runProviderTest(args[1:])
	default:
		return fmt.Errorf("unknown provider command %q", args[0])
	}
}

func providerService(home string) application.ProviderService {
	return application.NewProviderService(
		infrastructure.ProviderConfigFile{Path: filepath.Join(home, "providers.yaml")},
		infrastructure.Environment{},
		infrastructure.OpenAICompatibleFactory{},
	)
}

func runProviderList(args []string) error {
	fs := flag.NewFlagSet("provider list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	home := fs.String("home", defaultHome(), "GrandetAgent home directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("provider list accepts no provider name")
	}
	configs, err := providerService(*home).List()
	if err != nil {
		return err
	}
	for _, config := range configs {
		state := "disabled"
		if config.Enabled {
			state = "enabled"
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", config.Name, config.Type, state, config.BaseURL)
	}
	return nil
}

func runProviderTest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: grandet provider test <provider> [--home path]")
	}
	provider := args[0]
	fs := flag.NewFlagSet("provider test", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	home := fs.String("home", defaultHome(), "GrandetAgent home directory")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: grandet provider test <provider> [--home path]")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	health, err := providerService(*home).Test(ctx, provider)
	if err != nil {
		return err
	}
	if health.RequestID == "" {
		fmt.Printf("provider %s is healthy\n", provider)
		return nil
	}
	fmt.Printf("provider %s is healthy (request %s)\n", provider, health.RequestID)
	return nil
}

func defaultHome() string {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return ".grandet"
	}
	return filepath.Join(userHome, ".grandet")
}
