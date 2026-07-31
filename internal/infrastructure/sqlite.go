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
	return SQLiteMigrator{clock: clock, migrations: []Migration{{Version: 1, Up: createWorkspaceVersions}, {Version: 2, Up: createTelemetryTables}}}
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

func createTelemetryTables(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, status TEXT NOT NULL, active_execution_profile_id TEXT, policy_version TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE trajectories (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, status TEXT NOT NULL, prompt_hash TEXT NOT NULL, active_policy_version TEXT NOT NULL, selected_execution_profile_id TEXT NOT NULL, command_budget_usd REAL, started_at TEXT NOT NULL, completed_at TEXT, FOREIGN KEY(session_id) REFERENCES sessions(id))`,
		`CREATE TABLE tasks (id TEXT PRIMARY KEY, trajectory_id TEXT NOT NULL, status TEXT NOT NULL, task_family TEXT NOT NULL, difficulty INTEGER NOT NULL, created_at TEXT NOT NULL, completed_at TEXT, FOREIGN KEY(trajectory_id) REFERENCES trajectories(id))`,
		`CREATE TABLE steps (id TEXT PRIMARY KEY, task_id TEXT NOT NULL, sequence_no INTEGER NOT NULL, step_type TEXT NOT NULL, status TEXT NOT NULL, execution_profile_id TEXT, started_at TEXT, completed_at TEXT, FOREIGN KEY(task_id) REFERENCES tasks(id))`,
		`CREATE TABLE trajectory_events (id INTEGER PRIMARY KEY AUTOINCREMENT, trajectory_id TEXT NOT NULL, event_type TEXT NOT NULL, event_version INTEGER NOT NULL, payload_json TEXT NOT NULL, created_at TEXT NOT NULL, FOREIGN KEY(trajectory_id) REFERENCES trajectories(id))`,
		`CREATE TABLE model_calls (id TEXT PRIMARY KEY, trajectory_id TEXT NOT NULL, task_id TEXT NOT NULL, step_id TEXT NOT NULL, execution_profile_id TEXT NOT NULL, status TEXT NOT NULL, provider_request_id TEXT, input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER, ttft_ms INTEGER, total_latency_ms INTEGER, actual_cost_usd REAL, normalized_error_type TEXT, started_at TEXT NOT NULL, completed_at TEXT, FOREIGN KEY(trajectory_id) REFERENCES trajectories(id), FOREIGN KEY(task_id) REFERENCES tasks(id), FOREIGN KEY(step_id) REFERENCES steps(id))`,
		`CREATE TABLE tool_calls (id TEXT PRIMARY KEY, trajectory_id TEXT NOT NULL, task_id TEXT NOT NULL, step_id TEXT NOT NULL, tool_name TEXT NOT NULL, status TEXT NOT NULL, outcome TEXT, latency_ms INTEGER, error_type TEXT, created_at TEXT NOT NULL, FOREIGN KEY(trajectory_id) REFERENCES trajectories(id), FOREIGN KEY(task_id) REFERENCES tasks(id), FOREIGN KEY(step_id) REFERENCES steps(id))`,
		`CREATE TABLE validation_results (id TEXT PRIMARY KEY, trajectory_id TEXT NOT NULL, task_id TEXT NOT NULL, step_id TEXT, validator_type TEXT NOT NULL, status TEXT NOT NULL, created_at TEXT NOT NULL, FOREIGN KEY(trajectory_id) REFERENCES trajectories(id), FOREIGN KEY(task_id) REFERENCES tasks(id), FOREIGN KEY(step_id) REFERENCES steps(id))`,
		`CREATE TRIGGER trajectory_events_no_update BEFORE UPDATE ON trajectory_events BEGIN SELECT RAISE(ABORT, 'trajectory events are append-only'); END`,
		`CREATE TRIGGER trajectory_events_no_delete BEFORE DELETE ON trajectory_events BEGIN SELECT RAISE(ABORT, 'trajectory events are append-only'); END`,
		`CREATE INDEX trajectories_started_at ON trajectories(started_at)`,
		`CREATE INDEX trajectories_session_profile ON trajectories(session_id, selected_execution_profile_id, status)`,
		`CREATE INDEX tasks_trajectory_id ON tasks(trajectory_id)`,
		`CREATE INDEX trajectory_events_trajectory_id ON trajectory_events(trajectory_id, id)`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

var _ application.WorkspaceDatabase = SQLiteMigrator{}
