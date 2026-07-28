package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestRunAndAnalyzeBaseline(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	if err := run([]string{"init", "--home", home}); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if err := run([]string{"run", "--home", home, "--profile", "fixed-profile", "measure this"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "Profile: fixed-profile\nStatus: completed\n") {
		t.Fatalf("run output = %q", output)
	}
	output = captureStdout(t, func() {
		if err := run([]string{"analyze", "cost", "--home", home, "--last", "7d", "--profile", "fixed-profile", "--outcome", "completed"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "Trajectories: 1\n") || !strings.Contains(output, "Known provider cost: unknown\n") {
		t.Fatalf("cost report = %q", output)
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
