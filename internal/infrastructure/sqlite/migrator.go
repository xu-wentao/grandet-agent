package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	_ "github.com/mattn/go-sqlite3"
	"github.com/xu-wentao/grandet-agent/internal/application"
)

type Migration struct {
	Version int
	Up      func(context.Context, *sql.Tx) error
}

type Migrator struct{}

func (Migrator) Migrate(ctx context.Context, databasePath string, versions application.WorkspaceVersions) error {
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := ApplyMigrations(ctx, db, migrations()); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO workspace_versions (kind, version, recorded_at) VALUES
			('config', ?, ?),
			('policy', ?, ?)
		ON CONFLICT(kind, version) DO NOTHING`, versions.Config, versions.At, versions.Policy, versions.At)
	return err
}

func ApplyMigrations(ctx context.Context, db *sql.DB, migrations []Migration) error {
	current, exists, err := currentVersion(ctx, db)
	if err != nil {
		return err
	}
	if !exists {
		current = 0
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	for _, migration := range migrations {
		if migration.Version <= current {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		err = migration.Up(ctx, tx)
		if err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_versions (version, applied_at) VALUES (?, datetime('now'))", migration.Version)
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", migration.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.Version, err)
		}
		current = migration.Version
	}
	return nil
}

func currentVersion(ctx context.Context, db *sql.DB) (int, bool, error) {
	var name string
	err := db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'schema_versions'").Scan(&name)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	var version int
	if err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_versions").Scan(&version); err != nil {
		return 0, false, err
	}
	return version, true, nil
}

func migrations() []Migration {
	return []Migration{{Version: 1, Up: func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			CREATE TABLE schema_versions (
				version INTEGER PRIMARY KEY,
				applied_at TEXT NOT NULL
			);
			CREATE TABLE workspace_versions (
				kind TEXT NOT NULL,
				version TEXT NOT NULL,
				recorded_at TEXT NOT NULL,
				PRIMARY KEY (kind, version)
			);`)
		return err
	}}}
}
