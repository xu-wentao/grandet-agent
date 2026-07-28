package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/domain"
	"github.com/xu-wentao/grandet-agent/internal/infrastructure"
)

func runAnalyze(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("analyze requires cost or task-distribution")
	}
	switch args[0] {
	case "cost":
		return analyzeCost(args[1:])
	case "task-distribution":
		return analyzeTaskDistribution(args[1:])
	default:
		return fmt.Errorf("unknown analyze report %q", args[0])
	}
}

func analyzeCost(args []string) error {
	filter, repository, err := analyzeInputs("cost", args)
	if err != nil {
		return err
	}
	report, err := repository.CostReport(context.Background(), filter)
	if err != nil {
		return fmt.Errorf("read cost report: %w", err)
	}
	fmt.Printf("Trajectories: %d\nCompleted: %d\nFailed: %d\nIn progress: %d\nModel calls: %d\n", report.Trajectories, report.Completed, report.Failed, report.InProgress, report.ModelCalls)
	if report.KnownCostUSD == nil {
		fmt.Println("Known provider cost: unknown")
	} else {
		fmt.Printf("Known provider cost: $%.6f\n", *report.KnownCostUSD)
	}
	return nil
}

func analyzeTaskDistribution(args []string) error {
	filter, repository, err := analyzeInputs("task-distribution", args)
	if err != nil {
		return err
	}
	distribution, err := repository.TaskDistribution(context.Background(), filter)
	if err != nil {
		return fmt.Errorf("read task distribution: %w", err)
	}
	if len(distribution) == 0 {
		fmt.Println("No trajectories matched.")
		return nil
	}
	for _, entry := range distribution {
		fmt.Printf("%s: %d\n", entry.TaskFamily, entry.Count)
	}
	return nil
}

func analyzeInputs(name string, args []string) (domain.ReportFilter, infrastructure.SQLiteTelemetryRepository, error) {
	fs := flag.NewFlagSet("analyze "+name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var home, last, session, profile, outcome string
	fs.StringVar(&home, "home", "", "GrandetAgent home directory")
	fs.StringVar(&last, "last", "", "Only trajectories started in the last duration, for example 7d")
	fs.StringVar(&session, "session", "", "Only a session")
	fs.StringVar(&profile, "profile", "", "Only an explicit profile")
	fs.StringVar(&outcome, "outcome", "", "Only COMPLETED, FAILED, or RUNNING trajectories")
	if err := fs.Parse(args); err != nil {
		return domain.ReportFilter{}, infrastructure.SQLiteTelemetryRepository{}, err
	}
	if fs.NArg() != 0 {
		return domain.ReportFilter{}, infrastructure.SQLiteTelemetryRepository{}, fmt.Errorf("unexpected analyze arguments: %s", strings.Join(fs.Args(), " "))
	}
	duration, err := parseLast(last)
	if err != nil {
		return domain.ReportFilter{}, infrastructure.SQLiteTelemetryRepository{}, err
	}
	home, err = grandetHome(home)
	if err != nil {
		return domain.ReportFilter{}, infrastructure.SQLiteTelemetryRepository{}, err
	}
	path := filepath.Join(home, "grandet.db")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return domain.ReportFilter{}, infrastructure.SQLiteTelemetryRepository{}, fmt.Errorf("workspace is not initialized; run grandet init --home %s", home)
		}
		return domain.ReportFilter{}, infrastructure.SQLiteTelemetryRepository{}, fmt.Errorf("check workspace database: %w", err)
	}
	return domain.ReportFilter{Since: time.Now().UTC().Add(-duration), SessionID: session, ProfileID: profile, Outcome: strings.ToUpper(outcome)}, infrastructure.NewSQLiteTelemetryRepository(path, infrastructure.Clock{}), nil
}

func parseLast(value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("--last is required")
	}
	if strings.HasSuffix(value, "d") {
		var days int
		if _, err := fmt.Sscanf(strings.TrimSuffix(value, "d"), "%d", &days); err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid --last %q", value)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid --last %q", value)
	}
	return duration, nil
}
