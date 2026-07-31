package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/infrastructure"
)

func runModel(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: grandet model <sync|list|enable|disable|quarantine>")
	}
	switch args[0] {
	case "sync":
		return runModelSync(args[1:])
	case "list":
		return runModelList(args[1:])
	case "enable", "disable", "quarantine":
		return runModelState(args[0], args[1:])
	default:
		return fmt.Errorf("unknown model command %q", args[0])
	}
}

func runProfile(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: grandet profile <list|show>")
	}
	switch args[0] {
	case "list":
		return runProfileList(args[1:])
	case "show":
		return runProfileShow(args[1:])
	default:
		return fmt.Errorf("unknown profile command %q", args[0])
	}
}

func registryInputs(name string, args []string) (string, infrastructure.SQLiteRegistry, []application.ProviderConfig, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	home := fs.String("home", defaultHome(), "GrandetAgent home directory")
	if err := fs.Parse(args); err != nil {
		return "", infrastructure.SQLiteRegistry{}, nil, err
	}
	if fs.NArg() != 0 {
		return "", infrastructure.SQLiteRegistry{}, nil, fmt.Errorf("unexpected %s arguments", name)
	}
	path := filepath.Join(*home, "grandet.db")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", infrastructure.SQLiteRegistry{}, nil, fmt.Errorf("workspace is not initialized; run grandet init --home %s", *home)
		}
		return "", infrastructure.SQLiteRegistry{}, nil, fmt.Errorf("check workspace database: %w", err)
	}
	clock := infrastructure.Clock{}
	if err := infrastructure.NewSQLiteMigrator(clock).Migrate(path); err != nil {
		return "", infrastructure.SQLiteRegistry{}, nil, fmt.Errorf("migrate registry: %w", err)
	}
	configs, err := (infrastructure.ProviderConfigFile{Path: filepath.Join(*home, "providers.yaml")}).Load()
	if err != nil {
		return "", infrastructure.SQLiteRegistry{}, nil, err
	}
	registry := infrastructure.NewSQLiteRegistry(path, clock)
	if err := registry.UpsertProviders(context.Background(), configs); err != nil {
		return "", infrastructure.SQLiteRegistry{}, nil, err
	}
	if err := registry.ImportManualProfiles(context.Background(), filepath.Join(*home, "models.yaml")); err != nil {
		return "", infrastructure.SQLiteRegistry{}, nil, err
	}
	return *home, registry, configs, nil
}

func runModelSync(args []string) error {
	fs := flag.NewFlagSet("model sync", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	providerName := fs.String("provider", "", "Configured provider to synchronize")
	home := fs.String("home", defaultHome(), "GrandetAgent home directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *providerName == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: grandet model sync --provider <provider> [--home path]")
	}
	_, registry, configs, err := registryInputs("model sync", []string{"--home", *home})
	if err != nil {
		return err
	}
	var selected application.ProviderConfig
	for _, config := range configs {
		if config.Name == *providerName {
			selected = config
			break
		}
	}
	if selected.Name == "" {
		return fmt.Errorf("provider %q is not configured", *providerName)
	}
	models, err := providerService(*home).ListModels(context.Background(), *providerName)
	if err != nil {
		return err
	}
	if err := registry.Sync(context.Background(), selected, models); err != nil {
		return err
	}
	fmt.Printf("synchronized %d models from %s\n", len(models), *providerName)
	return nil
}

func runModelList(args []string) error {
	_, registry, _, err := registryInputs("model list", args)
	if err != nil {
		return err
	}
	models, err := registry.ListModels(context.Background())
	if err != nil {
		return err
	}
	for _, model := range models {
		price := "unknown"
		if model.PriceKnown {
			price = "known"
		}
		fmt.Printf("%s\t%s\t%t\t%t\t%s\n", model.ID, model.LifecycleState, model.Enabled, model.IsFree, price)
	}
	return nil
}

func runModelState(command string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: grandet model %s <model-id> [--home path]", command)
	}
	modelID := args[0]
	_, registry, _, err := registryInputs("model "+command, args[1:])
	if err != nil {
		return err
	}
	state := map[string]string{"enable": "ACTIVE", "disable": "DISABLED", "quarantine": "QUARANTINED"}[command]
	if err := registry.SetModelState(context.Background(), modelID, state); err != nil {
		return err
	}
	fmt.Printf("model %s is %s\n", modelID, state)
	return nil
}

func runProfileList(args []string) error {
	_, registry, _, err := registryInputs("profile list", args)
	if err != nil {
		return err
	}
	profiles, err := registry.ListExecutionProfiles(context.Background())
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		fmt.Printf("%s\t%s/%s\t%s\t%d\t%t\n", profile.ID, profile.Provider, profile.Model, profile.ReasoningMode, profile.MaxOutputTokens, profile.EligibleForAutomaticRouting(false))
	}
	return nil
}

func runProfileShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: grandet profile show <profile-id> [--home path]")
	}
	profileID := args[0]
	_, registry, _, err := registryInputs("profile show", args[1:])
	if err != nil {
		return err
	}
	profile, err := registry.ExecutionProfile(context.Background(), profileID)
	if err != nil {
		return err
	}
	fmt.Printf("ID: %s\nProvider: %s\nModel: %s\nReasoning: %s\nMax output tokens: %d\nTool calling: %t\nJSON output: %t\nVision: %t\nRetry policy: %s\nQuality tier: %s\nEnabled: %t\nLifecycle: %s\nPrice known: %t\n", profile.ID, profile.Provider, profile.Model, profile.ReasoningMode, profile.MaxOutputTokens, profile.Capabilities.ToolCalling, profile.Capabilities.JSONOutput, profile.Capabilities.Vision, profile.RetryPolicy, profile.QualityTier, profile.Enabled, profile.LifecycleState, profile.PriceKnown)
	return nil
}
