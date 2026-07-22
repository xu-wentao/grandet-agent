package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var dryRun bool
	var force bool
	var home string

	fs.BoolVar(&dryRun, "dry-run", false, "Print files and directories without creating them")
	fs.BoolVar(&force, "force", false, "Overwrite existing config files")
	fs.StringVar(&home, "home", "", "GrandetAgent home directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get user home: %w", err)
		}
		home = filepath.Join(userHome, ".grandet")
	}

	paths := []string{
		home,
		filepath.Join(home, "logs"),
		filepath.Join(home, "traces"),
		filepath.Join(home, "cache"),
		filepath.Join(home, "policies"),
		filepath.Join(home, "evals"),
		filepath.Join(home, "evals", "golden"),
		filepath.Join(home, "evals", "regression"),
		filepath.Join(home, "evals", "safety"),
	}

	files := map[string]string{
		filepath.Join(home, "config.yaml"):               defaultConfigYAML,
		filepath.Join(home, "providers.yaml"):            defaultProvidersYAML,
		filepath.Join(home, "models.yaml"):               defaultModelsYAML,
		filepath.Join(home, "user-profile.yaml"):         defaultUserProfileYAML,
		filepath.Join(home, "policies", "stingy-v1.yaml"): defaultPolicyYAML,
	}

	if dryRun {
		fmt.Println("GrandetAgent init dry run")
		for _, p := range paths {
			fmt.Printf("create dir: %s\n", p)
		}
		for p := range files {
			fmt.Printf("create file: %s\n", p)
		}
		return nil
	}

	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", p, err)
		}
	}

	for p, content := range files {
		if _, err := os.Stat(p); err == nil && !force {
			fmt.Printf("skip existing file: %s\n", p)
			continue
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write file %s: %w", p, err)
		}
		fmt.Printf("created file: %s\n", p)
	}

	fmt.Printf("GrandetAgent workspace initialized at %s\n", home)
	return nil
}
