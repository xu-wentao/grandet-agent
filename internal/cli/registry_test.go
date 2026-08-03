package cli

import (
	"os"
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

func TestProfileListUsesEffectiveRoutingEligibility(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	if err := run([]string{"init", "--home", home}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "providers.yaml"), []byte(`schema_version: 1
providers:
  test:
    type: openai_compatible
    base_url: https://example.test/v1
    enabled: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "models.yaml"), []byte(`models:
  - id: test/model
    provider: test
    upstream_name: test-model
    enabled: true
    is_free: true
    lifecycle_state: ACTIVE
execution_profiles:
  - id: test-profile
    model: test/model
    enabled: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	list := func() string {
		return captureStdout(t, func() {
			if err := run([]string{"profile", "list", "--home", home}); err != nil {
				t.Fatal(err)
			}
		})
	}
	if output := list(); !strings.Contains(output, "test-profile\ttest/test-model\tdisabled\t0\ttrue\n") {
		t.Fatalf("profile list = %q", output)
	}
	if err := os.WriteFile(filepath.Join(home, "providers.yaml"), []byte(`schema_version: 1
providers:
  test:
    type: openai_compatible
    base_url: https://example.test/v1
    enabled: false
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if output := list(); !strings.Contains(output, "test-profile\ttest/test-model\tdisabled\t0\tfalse\n") {
		t.Fatalf("disabled provider profile list = %q", output)
	}
	if err := os.WriteFile(filepath.Join(home, "providers.yaml"), []byte(`schema_version: 1
providers:
  test:
    type: openai_compatible
    base_url: https://example.test/v1
    enabled: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"model", "disable", "test/model", "--home", home}); err != nil {
		t.Fatal(err)
	}
	if output := list(); !strings.Contains(output, "test-profile\ttest/test-model\tdisabled\t0\tfalse\n") {
		t.Fatalf("disabled model profile list = %q", output)
	}
}
