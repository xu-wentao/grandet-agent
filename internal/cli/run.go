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
	args, err := intersperseRunFlags(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var home, profile, session, taskFamily string
	var budget float64
	fs.StringVar(&home, "home", "", "GrandetAgent home directory")
	fs.StringVar(&profile, "profile", "", "Explicit execution profile")
	fs.StringVar(&session, "session", "", "Resume an existing session")
	fs.StringVar(&taskFamily, "task-family", "general_qa", "Measured task family")
	fs.Float64Var(&budget, "max-cost-usd", 0, "Command budget in USD")
	if err := fs.Parse(args); err != nil {
		return err
	}
	prompt := strings.Join(fs.Args(), " ")
	if prompt == "" {
		return fmt.Errorf("run requires a prompt")
	}
	home, err = grandetHome(home)
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
	executor, err := infrastructure.LoadProviderExecutor(home, profile)
	if err != nil {
		return err
	}
	clock := infrastructure.Clock{}
	runner := application.NewBaselineRunner(infrastructure.NewSQLiteTelemetryRepository(databasePath, clock), clock, infrastructure.IDGenerator{}, executor)
	var commandBudget *float64
	if budget > 0 {
		commandBudget = &budget
	}
	run, result, err := runner.Execute(context.Background(), application.RunOptions{SessionID: session, ProfileID: profile, TaskFamily: taskFamily, Prompt: prompt, CommandBudgetUS: commandBudget})
	if err != nil {
		return err
	}
	fmt.Printf("Trajectory: %s\nSession: %s\nProfile: %s\nStatus: completed\n", run.TrajectoryID, run.SessionID, run.ProfileID)
	if result.Output != "" {
		fmt.Println(result.Output)
	}
	return nil
}

func intersperseRunFlags(args []string) ([]string, error) {
	withValue := map[string]bool{"--home": true, "--profile": true, "--session": true, "--task-family": true, "--max-cost-usd": true}
	var flags, prompt []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		name := argument
		if equal := strings.IndexByte(argument, '='); equal >= 0 {
			name = argument[:equal]
		}
		if !withValue[name] {
			prompt = append(prompt, argument)
			continue
		}
		flags = append(flags, argument)
		if !strings.Contains(argument, "=") {
			if index+1 == len(args) {
				return nil, fmt.Errorf("flag needs an argument: %s", name)
			}
			index++
			flags = append(flags, args[index])
		}
	}
	return append(flags, prompt...), nil
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
