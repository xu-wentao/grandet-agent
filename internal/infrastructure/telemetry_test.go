package infrastructure

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

func TestTelemetryStartIsAtomicAndCrashSafe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grandet.db")
	if err := NewSQLiteMigrator(testClock{}).Migrate(path); err != nil {
		t.Fatal(err)
	}
	repository := NewSQLiteTelemetryRepository(path, testClock{})
	started := time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC)
	run := domain.BaselineRun{SessionID: "session-1", TrajectoryID: "trajectory-1", TaskID: "task-1", StepID: "step-1", ProfileID: "fixed", TaskFamily: "summarization", PolicyVersion: "stingy-v1", PromptHash: "hash", StartedAt: started}
	if err := repository.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for table := range map[string]bool{"sessions": true, "trajectories": true, "tasks": true, "steps": true, "trajectory_events": true} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s rows = %d, %v", table, count, err)
		}
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM trajectories WHERE id = 'trajectory-1'`).Scan(&status); err != nil || status != "RUNNING" {
		t.Fatalf("partial trajectory status = %q, %v", status, err)
	}

	duplicate := run
	duplicate.SessionID = "session-should-rollback"
	if err := repository.Start(context.Background(), duplicate); err == nil {
		t.Fatal("expected duplicate trajectory to fail")
	}
	var sessions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatalf("failed transaction left session: %d, %v", sessions, err)
	}
}

func TestTelemetryEventsAreAppendOnlyAndUsageStaysUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grandet.db")
	if err := NewSQLiteMigrator(testClock{}).Migrate(path); err != nil {
		t.Fatal(err)
	}
	repository := NewSQLiteTelemetryRepository(path, testClock{})
	started := time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC)
	run := domain.BaselineRun{SessionID: "session-1", TrajectoryID: "trajectory-1", TaskID: "task-1", StepID: "step-1", ProfileID: "fixed", TaskFamily: "summarization", PolicyVersion: "stingy-v1", PromptHash: "hash", StartedAt: started}
	if err := repository.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	call := domain.ModelCall{ID: "call-1", TrajectoryID: run.TrajectoryID, TaskID: run.TaskID, StepID: run.StepID, ProfileID: run.ProfileID, StartedAt: started, CompletedAt: started.Add(time.Second)}
	if err := repository.StartModelCall(context.Background(), call); err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteModelCall(context.Background(), call); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE trajectory_events SET event_type = 'changed' WHERE id = 1`); err == nil {
		t.Fatal("expected append-only trigger to reject update")
	}
	var inputTokens sql.NullInt64
	if err := db.QueryRow(`SELECT input_tokens FROM model_calls WHERE id = 'call-1'`).Scan(&inputTokens); err != nil || inputTokens.Valid {
		t.Fatalf("missing input tokens were fabricated: %#v, %v", inputTokens, err)
	}
	report, err := repository.CostReport(context.Background(), domain.ReportFilter{Since: started.Add(-time.Second), ProfileID: "fixed"})
	if err != nil {
		t.Fatal(err)
	}
	if report.ModelCalls != 1 || report.KnownCostUSD != nil {
		t.Fatalf("report = %#v", report)
	}
}

func TestCallOwnershipComesFromStoredRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grandet.db")
	if err := NewSQLiteMigrator(testClock{}).Migrate(path); err != nil {
		t.Fatal(err)
	}
	repository := NewSQLiteTelemetryRepository(path, testClock{})
	started := time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC)
	first := domain.BaselineRun{SessionID: "session-1", TrajectoryID: "trajectory-1", TaskID: "task-1", StepID: "step-1", ProfileID: "fixed", TaskFamily: "summarization", PolicyVersion: "stingy-v1", PromptHash: "hash", StartedAt: started}
	second := domain.BaselineRun{SessionID: "session-2", TrajectoryID: "trajectory-2", TaskID: "task-2", StepID: "step-2", ProfileID: "fixed", TaskFamily: "debugging", PolicyVersion: "stingy-v1", PromptHash: "hash", StartedAt: started}
	for _, run := range []domain.BaselineRun{first, second} {
		if err := repository.Start(context.Background(), run); err != nil {
			t.Fatal(err)
		}
	}
	model := domain.ModelCall{ID: "model-1", TrajectoryID: first.TrajectoryID, TaskID: first.TaskID, StepID: first.StepID, ProfileID: first.ProfileID, StartedAt: started, CompletedAt: started}
	if err := repository.StartModelCall(context.Background(), model); err != nil {
		t.Fatal(err)
	}
	model.TrajectoryID = second.TrajectoryID
	if err := repository.CompleteModelCall(context.Background(), model); err != nil {
		t.Fatal(err)
	}
	tool := domain.ToolCall{ID: "tool-1", TrajectoryID: first.TrajectoryID, TaskID: first.TaskID, StepID: first.StepID, Name: "test", CreatedAt: started}
	if err := repository.StartToolCall(context.Background(), tool); err != nil {
		t.Fatal(err)
	}
	tool.TrajectoryID = second.TrajectoryID
	if err := repository.CompleteToolCall(context.Background(), tool); err != nil {
		t.Fatal(err)
	}
	if err := repository.StartModelCall(context.Background(), domain.ModelCall{ID: "bad", TrajectoryID: first.TrajectoryID, TaskID: second.TaskID, StepID: second.StepID, ProfileID: first.ProfileID, StartedAt: started}); err == nil {
		t.Fatal("expected mismatched model ownership to fail")
	}
	if err := repository.StartToolCall(context.Background(), domain.ToolCall{ID: "bad", TrajectoryID: first.TrajectoryID, TaskID: second.TaskID, StepID: second.StepID, Name: "test", CreatedAt: started}); err == nil {
		t.Fatal("expected mismatched tool ownership to fail")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, eventType := range []string{"model_call_completed", "tool_call_completed"} {
		var trajectoryID string
		if err := db.QueryRow(`SELECT trajectory_id FROM trajectory_events WHERE event_type = ?`, eventType).Scan(&trajectoryID); err != nil || trajectoryID != first.TrajectoryID {
			t.Fatalf("%s trajectory = %q, %v", eventType, trajectoryID, err)
		}
	}
}
