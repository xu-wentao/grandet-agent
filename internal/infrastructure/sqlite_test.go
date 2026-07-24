package infrastructure

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

type testClock struct{}

func (testClock) Now() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

func TestSQLiteMigrationRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grandet.db")
	migrator := SQLiteMigrator{clock: testClock{}, migrations: []Migration{{Version: 1, Up: func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE must_rollback (id INTEGER)`); err != nil {
			return err
		}
		return errors.New("stop")
	}}}}
	if err := migrator.Migrate(path); err == nil {
		t.Fatal("expected migration error")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_versions`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed migration was recorded: %d, %v", count, err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'must_rollback')`).Scan(&exists); err != nil || exists {
		t.Fatalf("migration changes were not rolled back: %t, %v", exists, err)
	}
}
