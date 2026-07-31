package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

func TestJSONErrorOutputUsesStableCode(t *testing.T) {
	var stderr bytes.Buffer
	if code := execute([]string{"--output", "json", "missing"}, &stderr); code != 3 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	var output struct {
		Error struct {
			Code      string `json:"code"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Error.Code != "validation_error" || output.Error.Retryable {
		t.Fatalf("error output = %#v", output.Error)
	}
}

func TestJSONErrorOutputRedactsProviderDiagnosticAndKeepsCorrelation(t *testing.T) {
	var stderr bytes.Buffer
	writeError(&stderr, domain.NewProviderError(domain.CodeProviderRateLimited, "provider failed", true, domain.Correlation{
		TrajectoryID:      "trj-1",
		StepID:            "step-1",
		ProviderRequestID: "req-1",
		PolicyVersion:     "stingy-v1",
	}, domain.ProviderDiagnostic{Message: "Authorization: Bearer sk_secret"}, nil), "json")

	if strings.Contains(stderr.String(), "sk_secret") {
		t.Fatalf("JSON error leaked secret: %s", stderr.String())
	}
	var output struct {
		Error struct {
			Code          string `json:"code"`
			TrajectoryID  string `json:"trajectory_id"`
			StepID        string `json:"step_id"`
			PolicyVersion string `json:"policy_version"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Error.Code != "provider_rate_limited" || output.Error.TrajectoryID != "trj-1" || output.Error.StepID != "step-1" || output.Error.PolicyVersion != "stingy-v1" {
		t.Fatalf("error output = %#v", output.Error)
	}
}

func TestJSONErrorOutputSuppressesFlagDiagnosticsAndSecrets(t *testing.T) {
	var stderr bytes.Buffer
	if code := execute([]string{"--output", "json", "init", "--force=sk-proj-secret"}, &stderr); code != 3 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "sk-proj-secret") {
		t.Fatalf("JSON error leaked secret: %s", stderr.String())
	}
	var output struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &output); err != nil {
		t.Fatalf("JSON output includes flag diagnostics: %s", stderr.String())
	}
	if output.Error.Code != "validation_error" {
		t.Fatalf("error output = %#v", output.Error)
	}
}

func TestInitDryRunPrintsPlanWithoutCreatingWorkspace(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	var err error
	output := captureStdout(t, func() {
		err = run([]string{"init", "--home", home, "--dry-run"})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		home,
		filepath.Join(home, "logs"),
		filepath.Join(home, "traces"),
		filepath.Join(home, "cache"),
		filepath.Join(home, "policies"),
		filepath.Join(home, "evals"),
		filepath.Join(home, "evals", "golden"),
		filepath.Join(home, "evals", "regression"),
		filepath.Join(home, "evals", "safety"),
	} {
		if !strings.Contains(output, "create dir: "+path+"\n") {
			t.Errorf("dry-run output missing directory %s:\n%s", path, output)
		}
	}
	for _, path := range []string{
		filepath.Join(home, "config.yaml"),
		filepath.Join(home, "providers.yaml"),
		filepath.Join(home, "models.yaml"),
		filepath.Join(home, "user-profile.yaml"),
		filepath.Join(home, "policies", "stingy-v1.yaml"),
		filepath.Join(home, "grandet.db"),
	} {
		if !strings.Contains(output, "create file: "+path+"\n") {
			t.Errorf("dry-run output missing file %s:\n%s", path, output)
		}
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("dry-run created workspace: %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}
