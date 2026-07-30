package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/infrastructure"
)

const helpText = `GrandetAgent is a local-first, cost-aware Agent CLI.

Usage:
  grandet <command> [options]

Available Commands:
  init      Initialize local GrandetAgent workspace
  provider  List or test configured providers
  version   Print GrandetAgent version
  help      Print this help message

Examples:
  grandet init
  grandet init --dry-run
  grandet init --home ./tmp/.grandet
`

func Execute() {
	if code := execute(os.Args[1:], os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func run(args []string) error {
	args, _, err := outputFormat(args)
	if err != nil {
		return err
	}
	return runCommand(args)
}

func execute(args []string, stderr io.Writer) int {
	args, format, err := outputFormat(args)
	if err == nil {
		err = runCommand(args)
	}
	if err == nil {
		return 0
	}
	normalized := application.NormalizeError(err)
	writeError(stderr, normalized, format)
	return exitCode(string(normalized.Code))
}

func runCommand(args []string) error {
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
	case "provider":
		return runProvider(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText)
	}
}

func outputFormat(args []string) ([]string, string, error) {
	format := "text"
	remaining := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--output" {
			if index+1 == len(args) {
				return nil, format, fmt.Errorf("--output requires a value")
			}
			index++
			format = args[index]
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--output="); ok {
			format = value
			continue
		}
		remaining = append(remaining, arg)
	}
	if format != "text" && format != "json" {
		return nil, format, fmt.Errorf("unsupported output format %q", format)
	}
	return remaining, format, nil
}

func writeError(writer io.Writer, err error, format string) {
	normalized := application.NormalizeError(err)
	message := infrastructure.Redact(normalized.Message)
	if format != "json" {
		fmt.Fprintf(writer, "%s: %s\n", normalized.Code, message)
		return
	}
	type providerOutput struct {
		Provider   string `json:"provider,omitempty"`
		StatusCode int    `json:"status_code,omitempty"`
		RequestID  string `json:"request_id,omitempty"`
		Message    string `json:"message,omitempty"`
	}
	type errorOutput struct {
		Code              string          `json:"code"`
		Message           string          `json:"message"`
		Retryable         bool            `json:"retryable"`
		TrajectoryID      string          `json:"trajectory_id,omitempty"`
		StepID            string          `json:"step_id,omitempty"`
		ProviderRequestID string          `json:"provider_request_id,omitempty"`
		PolicyVersion     string          `json:"policy_version,omitempty"`
		Provider          *providerOutput `json:"provider,omitempty"`
	}
	output := struct {
		Error errorOutput `json:"error"`
	}{Error: errorOutput{
		Code:              string(normalized.Code),
		Message:           message,
		Retryable:         normalized.Retryable,
		TrajectoryID:      normalized.Correlation.TrajectoryID,
		StepID:            normalized.Correlation.StepID,
		ProviderRequestID: normalized.Correlation.ProviderRequestID,
		PolicyVersion:     normalized.Correlation.PolicyVersion,
	}}
	if normalized.Provider != nil {
		output.Error.Provider = &providerOutput{
			Provider:   infrastructure.Redact(normalized.Provider.Provider),
			StatusCode: normalized.Provider.StatusCode,
			RequestID:  infrastructure.Redact(normalized.Provider.RequestID),
			Message:    infrastructure.Redact(normalized.Provider.Message),
		}
	}
	_ = json.NewEncoder(writer).Encode(output)
}

func exitCode(code string) int {
	switch code {
	case "configuration_error":
		return 2
	case "validation_error":
		return 3
	case "budget_exceeded":
		return 4
	case "routing_failed":
		return 5
	case "provider_timeout", "provider_rate_limited", "provider_unavailable", "provider_rejected":
		return 6
	case "persistence_failure":
		return 7
	case "policy_violation":
		return 8
	default:
		return 1
	}
}
