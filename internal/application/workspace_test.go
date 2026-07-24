package application_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/infrastructure"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

type fixedIDs struct{}

func (fixedIDs) New() string { return "test-id" }

func initializer() application.WorkspaceInitializer {
	clock := fixedClock{}
	return application.NewWorkspaceInitializer(infrastructure.Filesystem{}, infrastructure.NewSQLiteMigrator(clock), clock, fixedIDs{})
}

func TestWorkspaceInitialization(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	service := initializer()

	t.Run("fresh", func(t *testing.T) {
		result, err := service.Initialize(application.InitOptions{Home: home})
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range result.Plan.Directories {
			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				t.Fatalf("directory %s was not created: %v", path, err)
			}
		}
		for _, path := range result.Plan.Files {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("file %s was not created: %v", path, err)
			}
		}
		assertVersions(t, filepath.Join(home, "grandet.db"))
	})

	t.Run("repeat preserves generated files", func(t *testing.T) {
		config := filepath.Join(home, "config.yaml")
		if err := os.WriteFile(config, []byte("user: change\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Initialize(application.InitOptions{Home: home}); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(config)
		if err != nil || string(content) != "user: change\n" {
			t.Fatalf("config was overwritten: %q, %v", content, err)
		}
	})

	t.Run("force replaces defaults but keeps unrelated files", func(t *testing.T) {
		unrelated := filepath.Join(home, "notes.txt")
		if err := os.WriteFile(unrelated, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Initialize(application.InitOptions{Home: home, Force: true}); err != nil {
			t.Fatal(err)
		}
		config, err := os.ReadFile(filepath.Join(home, "config.yaml"))
		if err != nil || !strings.HasPrefix(string(config), "schema_version: 1\n") {
			t.Fatalf("config was not reset: %q, %v", config, err)
		}
		if content, err := os.ReadFile(unrelated); err != nil || string(content) != "keep" {
			t.Fatalf("unrelated file changed: %q, %v", content, err)
		}
	})
}

func TestWorkspaceInitializationDryRun(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	result, err := initializer().Initialize(application.InitOptions{Home: home, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Plan.Files) != 6 || result.Plan.Files[len(result.Plan.Files)-1] != filepath.Join(home, "grandet.db") {
		t.Fatalf("database missing from dry-run plan: %#v", result.Plan.Files)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("dry run created workspace: %v", err)
	}
}

func assertVersions(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var migrations, versions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_versions`).Scan(&migrations); err != nil || migrations != 1 {
		t.Fatalf("schema versions = %d, %v", migrations, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM workspace_versions`).Scan(&versions); err != nil || versions != 5 {
		t.Fatalf("workspace versions = %d, %v", versions, err)
	}
}
