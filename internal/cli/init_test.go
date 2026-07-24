package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestInitFreshCreatesWorkspaceAndVersions(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	if err := runInit([]string{"--home", home}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"config.yaml", "providers.yaml", "models.yaml", "user-profile.yaml", "policies/stingy-v1.yaml",
		"evals/golden", "evals/regression", "evals/safety", "grandet.db",
	} {
		if _, err := os.Stat(filepath.Join(home, path)); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
	db, err := sql.Open("sqlite3", filepath.Join(home, "grandet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var schemaVersions, workspaceVersions int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_versions").Scan(&schemaVersions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM workspace_versions").Scan(&workspaceVersions); err != nil {
		t.Fatal(err)
	}
	if schemaVersions != 1 || workspaceVersions != 2 {
		t.Fatalf("versions = schema:%d workspace:%d, want 1 and 2", schemaVersions, workspaceVersions)
	}
}

func TestInitPreservesFilesUnlessForced(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	if err := runInit([]string{"--home", home}); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(config, []byte("user: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(home, "policies", "custom.yaml")
	if err := os.WriteFile(unrelated, []byte("keep: me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit([]string{"--home", home}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(config); err != nil || string(got) != "user: value\n" {
		t.Fatalf("config after repeat = %q, %v", got, err)
	}
	if err := runInit([]string{"--home", home, "--force"}); err != nil {
		t.Fatal(err)
	}
	for path, settings := range map[string][]string{
		"config.yaml": {
			"cost_accounting:\n", "free_models:\n", "feedback:\n", "shadow_eval:\n", "golden_set:\n",
		},
		"models.yaml":             {"      - documentation\n      - summarization\n      - chinese\n"},
		"user-profile.yaml":       {"  - task_family: summarization\n"},
		"policies/stingy-v1.yaml": {"  prefer_reasoning_mode_change_before_model_switch: true\n"},
	} {
		got, err := os.ReadFile(filepath.Join(home, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, setting := range settings {
			if !strings.Contains(string(got), setting) {
				t.Fatalf("%s after force missing %q", path, setting)
			}
		}
	}
	if got, err := os.ReadFile(unrelated); err != nil || string(got) != "keep: me\n" {
		t.Fatalf("unrelated file = %q, %v", got, err)
	}
}

func TestInitDryRunDoesNotCreateWorkspace(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	if err := runInit([]string{"--home", home, "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("workspace exists after dry run: %v", err)
	}
}
