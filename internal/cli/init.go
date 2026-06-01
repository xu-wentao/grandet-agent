package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var dryRun bool
	var force bool
	var home string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize local GrandetAgent workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
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
				filepath.Join(home, "evals"),
				filepath.Join(home, "cache"),
			}

			files := map[string]string{
				filepath.Join(home, "config.yaml"):       defaultConfigYAML,
				filepath.Join(home, "providers.yaml"):    defaultProvidersYAML,
				filepath.Join(home, "models.yaml"):       defaultModelsYAML,
				filepath.Join(home, "user-profile.yaml"): defaultUserProfileYAML,
			}

			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "GrandetAgent init dry run")
				for _, p := range paths {
					fmt.Fprintf(cmd.OutOrStdout(), "create dir: %s\n", p)
				}
				for p := range files {
					fmt.Fprintf(cmd.OutOrStdout(), "create file: %s\n", p)
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
					fmt.Fprintf(cmd.OutOrStdout(), "skip existing file: %s\n", p)
					continue
				}
				if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
					return fmt.Errorf("write file %s: %w", p, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "created file: %s\n", p)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "GrandetAgent workspace initialized at %s\n", home)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print files and directories without creating them")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config files")
	cmd.Flags().StringVar(&home, "home", "", "GrandetAgent home directory")

	return cmd
}
