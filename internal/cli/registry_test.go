package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestModelAndProfileRegistryCommands(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	if err := run([]string{"init", "--home", home}); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if err := run([]string{"profile", "list", "--home", home}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "openai-mini-default") || !strings.Contains(output, "openai-mini-high-reasoning") {
		t.Fatalf("profile list = %q", output)
	}
	output = captureStdout(t, func() {
		if err := run([]string{"profile", "show", "openai-mini-high-reasoning", "--home", home}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "Reasoning: high") || !strings.Contains(output, "JSON output: true") {
		t.Fatalf("profile show = %q", output)
	}
	if err := run([]string{"model", "disable", "openai/gpt-5.4-mini", "--home", home}); err != nil {
		t.Fatal(err)
	}
	output = captureStdout(t, func() {
		if err := run([]string{"model", "list", "--home", home}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "openai/gpt-5.4-mini\tDISABLED\tfalse") {
		t.Fatalf("model list = %q", output)
	}
}
