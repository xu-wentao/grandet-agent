package cli

import (
	"fmt"
	"os"
)

const helpText = `GrandetAgent is a local-first, cost-aware Agent CLI.

Usage:
  grandet <command> [options]

Available Commands:
  init      Initialize local GrandetAgent workspace
  version   Print GrandetAgent version
  help      Print this help message

Examples:
  grandet init
  grandet init --dry-run
  grandet init --home ./tmp/.grandet
`

func Execute() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(helpText)
		return nil
	}

	switch args[0] {
	case "help", "--help", "-h":
		fmt.Print(helpText)
		return nil
	case "version":
		fmt.Println("grandet dev")
		return nil
	case "init":
		return runInit(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText)
	}
}
