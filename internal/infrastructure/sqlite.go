package infrastructure

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/domain"
	_ "modernc.org/sqlite"
)

type Migration struct {
	Version int
	Up      func(*sql.Tx) error
}

type SQLiteMigrator struct {
	clock      domain.Clock
	migrations []Migration
}

var _ application.WorkspaceDatabase = SQLiteMigrator{}

func NewSQLiteMigrator(clock domain.Clock) SQLiteMigrator {
	return SQLiteMigrator{clock: clock, migrations: []Migration{{Version: 1, Up: createWorkspaceVersions}}}
}

func (m SQLiteMigrator) Migrate(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_versions (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}
	migrations := append([]Migration(nil), m.migrations...)
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	for _, migration := range migrations {
		var applied bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_versions WHERE version = ?)`, migration.Version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := migration.Up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", migration.Version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_versions(version, applied_at) VALUES(?, ?)`, migration.Version, m.clock.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (m SQLiteMigrator) RecordVersions(path string, versions map[string]string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	for name, version := range versions {
		if _, err := db.Exec(`INSERT INTO workspace_versions(name, version, updated_at) VALUES(?, ?, ?) ON CONFLICT(name) DO UPDATE SET version = excluded.version, updated_at = excluded.updated_at`, name, version, m.clock.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return nil
}

func createWorkspaceVersions(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE workspace_versions (name TEXT PRIMARY KEY, version TEXT NOT NULL, updated_at TEXT NOT NULL)`)
	return err
}
