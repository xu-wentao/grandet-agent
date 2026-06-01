package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "grandet",
	Short: "GrandetAgent is a local-first, cost-aware Agent CLI",
	Long: `GrandetAgent is a local-first, cost-aware Agent CLI.

It routes tasks to the cheapest viable model first, uses fallback only when needed,
and learns from local user feedback over time.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newVersionCmd())
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print GrandetAgent version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "grandet dev")
		},
	}
}
