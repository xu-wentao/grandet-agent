package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/xu-wentao/grandet-agent/internal/application"
)

func TestMigratorCanRerun(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "grandet.db")
	migrator := Migrator{}
	versions := application.WorkspaceVersions{Config: "v2", Policy: "stingy-v1", At: "2026-07-24T00:00:00Z"}
	if err := migrator.Migrate(context.Background(), databasePath, versions); err != nil {
		t.Fatal(err)
	}
	if err := migrator.Migrate(context.Background(), databasePath, versions); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", databasePath)
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

func TestApplyMigrationsRollsBackFailure(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = ApplyMigrations(context.Background(), db, []Migration{{Version: 1, Up: func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "CREATE TABLE should_not_exist (id INTEGER)"); err != nil {
			return err
		}
		return errors.New("boom")
	}}})
	if err == nil {
		t.Fatal("expected migration failure")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'should_not_exist'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed migration left its table behind")
	}
}
