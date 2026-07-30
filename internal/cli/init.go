package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/infrastructure"
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
		home = defaultHome()
	}

	clock := infrastructure.Clock{}
	initializer := application.NewWorkspaceInitializer(infrastructure.Filesystem{}, infrastructure.NewSQLiteMigrator(clock), clock, infrastructure.IDGenerator{})
	result, err := initializer.Initialize(application.InitOptions{Home: home, DryRun: dryRun, Force: force})
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Println("GrandetAgent init dry run")
		for _, p := range result.Plan.Directories {
			fmt.Printf("create dir: %s\n", p)
		}
		for _, p := range result.Plan.Files {
			fmt.Printf("create file: %s\n", p)
		}
		return nil
	}
	for _, p := range result.Created {
		fmt.Printf("created file: %s\n", p)
	}

	fmt.Printf("GrandetAgent workspace initialized at %s\n", home)
	return nil
}
