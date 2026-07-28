package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/infrastructure"
)

func runBaseline(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var home, profile, session string
	var budget float64
	fs.StringVar(&home, "home", "", "GrandetAgent home directory")
	fs.StringVar(&profile, "profile", "", "Explicit execution profile")
	fs.StringVar(&session, "session", "", "Resume an existing session")
	fs.Float64Var(&budget, "max-cost-usd", 0, "Command budget in USD")
	if err := fs.Parse(args); err != nil {
		return err
	}
	prompt := strings.Join(fs.Args(), " ")
	if prompt == "" {
		return fmt.Errorf("run requires a prompt")
	}
	home, err := grandetHome(home)
	if err != nil {
		return err
	}
	databasePath := filepath.Join(home, "grandet.db")
	if _, err := os.Stat(databasePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workspace is not initialized; run grandet init --home %s", home)
		}
		return fmt.Errorf("check workspace database: %w", err)
	}
	clock := infrastructure.Clock{}
	runner := application.NewBaselineRunner(infrastructure.NewSQLiteTelemetryRepository(databasePath, clock), clock, infrastructure.IDGenerator{})
	var commandBudget *float64
	if budget > 0 {
		commandBudget = &budget
	}
	run, err := runner.Start(context.Background(), application.RunOptions{SessionID: session, ProfileID: profile, Prompt: prompt, CommandBudgetUS: commandBudget})
	if err != nil {
		return err
	}
	if err := runner.Complete(context.Background(), run.TrajectoryID); err != nil {
		return err
	}
	fmt.Printf("Trajectory: %s\nSession: %s\nProfile: %s\nStatus: completed\n", run.TrajectoryID, run.SessionID, run.ProfileID)
	fmt.Println("Provider execution is not configured; provider usage remains unknown.")
	return nil
}

func grandetHome(home string) (string, error) {
	if home != "" {
		return home, nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home: %w", err)
	}
	return filepath.Join(userHome, ".grandet"), nil
}
