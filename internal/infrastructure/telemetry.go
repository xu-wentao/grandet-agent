package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteTelemetryRepository struct {
	path  string
	clock domain.Clock
}

func NewSQLiteTelemetryRepository(path string, clock domain.Clock) SQLiteTelemetryRepository {
	return SQLiteTelemetryRepository{path: path, clock: clock}
}

func (r SQLiteTelemetryRepository) Start(ctx context.Context, run domain.BaselineRun) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := startRun(ctx, tx, run); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r SQLiteTelemetryRepository) AppendEvent(ctx context.Context, event domain.Event) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := appendEvent(ctx, tx, event.TrajectoryID, event.Type, event.Payload, timestamp(event.CreatedAt)); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r SQLiteTelemetryRepository) RecordRetry(ctx context.Context, trajectoryID, reason string, createdAt time.Time) error {
	return r.AppendEvent(ctx, domain.Event{TrajectoryID: trajectoryID, Type: "retry", Payload: map[string]any{"reason": reason}, CreatedAt: createdAt})
}

func (r SQLiteTelemetryRepository) RecordFallback(ctx context.Context, trajectoryID, reason string, createdAt time.Time) error {
	return r.AppendEvent(ctx, domain.Event{TrajectoryID: trajectoryID, Type: "fallback", Payload: map[string]any{"reason": reason}, CreatedAt: createdAt})
}

// StartModelCall creates its step-linked audit record before the provider is invoked.
func (r SQLiteTelemetryRepository) StartModelCall(ctx context.Context, call domain.ModelCall) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	started := timestamp(call.StartedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO model_calls(id, trajectory_id, task_id, step_id, execution_profile_id, status, started_at) VALUES(?, ?, ?, ?, ?, 'RUNNING', ?)`, call.ID, call.TrajectoryID, call.TaskID, call.StepID, call.ProfileID, started); err != nil {
		tx.Rollback()
		return err
	}
	if err := appendEvent(ctx, tx, call.TrajectoryID, "model_call_started", map[string]any{"model_call_id": call.ID, "step_id": call.StepID, "profile_id": call.ProfileID}, started); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r SQLiteTelemetryRepository) CompleteModelCall(ctx context.Context, call domain.ModelCall) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	status, eventType := "COMPLETED", "model_call_completed"
	if call.ErrorType != nil {
		status, eventType = "FAILED", "model_call_failed"
	}
	completed := timestamp(call.CompletedAt)
	result, err := tx.ExecContext(ctx, `UPDATE model_calls SET status = ?, provider_request_id = ?, input_tokens = ?, output_tokens = ?, reasoning_tokens = ?, ttft_ms = ?, total_latency_ms = ?, actual_cost_usd = ?, normalized_error_type = ?, completed_at = ? WHERE id = ? AND status = 'RUNNING'`, status, call.ProviderRequestID, call.InputTokens, call.OutputTokens, call.ReasoningTokens, call.TTFTMilliseconds, call.LatencyMilliseconds, call.ActualCostUSD, call.ErrorType, completed, call.ID)
	if err != nil {
		tx.Rollback()
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		tx.Rollback()
		if err != nil {
			return err
		}
		return fmt.Errorf("model call %s is not running", call.ID)
	}
	payload := map[string]any{"model_call_id": call.ID}
	optional(payload, "provider_request_id", call.ProviderRequestID)
	optional(payload, "input_tokens", call.InputTokens)
	optional(payload, "output_tokens", call.OutputTokens)
	optional(payload, "reasoning_tokens", call.ReasoningTokens)
	optional(payload, "ttft_ms", call.TTFTMilliseconds)
	optional(payload, "total_latency_ms", call.LatencyMilliseconds)
	optional(payload, "error_type", call.ErrorType)
	if err := appendEvent(ctx, tx, call.TrajectoryID, eventType, payload, completed); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// StartToolCall writes the start event before tool execution.
func (r SQLiteTelemetryRepository) StartToolCall(ctx context.Context, call domain.ToolCall) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	created := timestamp(call.CreatedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO tool_calls(id, trajectory_id, task_id, step_id, tool_name, status, created_at) VALUES(?, ?, ?, ?, ?, 'RUNNING', ?)`, call.ID, call.TrajectoryID, call.TaskID, call.StepID, call.Name, created); err != nil {
		tx.Rollback()
		return err
	}
	if err := appendEvent(ctx, tx, call.TrajectoryID, "tool_call_started", map[string]any{"tool_call_id": call.ID, "tool_name": call.Name, "step_id": call.StepID}, created); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r SQLiteTelemetryRepository) CompleteToolCall(ctx context.Context, call domain.ToolCall) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	status, eventType := "COMPLETED", "tool_call_completed"
	if call.ErrorType != nil {
		status, eventType = "FAILED", "tool_call_failed"
	}
	created := timestamp(call.CreatedAt)
	result, err := tx.ExecContext(ctx, `UPDATE tool_calls SET status = ?, outcome = ?, latency_ms = ?, error_type = ? WHERE id = ? AND status = 'RUNNING'`, status, call.Outcome, call.LatencyMS, call.ErrorType, call.ID)
	if err != nil {
		tx.Rollback()
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		tx.Rollback()
		if err != nil {
			return err
		}
		return fmt.Errorf("tool call %s is not running", call.ID)
	}
	payload := map[string]any{"tool_call_id": call.ID, "tool_name": call.Name, "outcome": call.Outcome}
	optional(payload, "latency_ms", call.LatencyMS)
	optional(payload, "error_type", call.ErrorType)
	if err := appendEvent(ctx, tx, call.TrajectoryID, eventType, payload, created); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r SQLiteTelemetryRepository) RecordToolCall(ctx context.Context, call domain.ToolCall) error {
	if err := r.StartToolCall(ctx, call); err != nil {
		return err
	}
	return r.CompleteToolCall(ctx, call)
}

func (r SQLiteTelemetryRepository) RecordValidation(ctx context.Context, result domain.ValidationResult) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	created := timestamp(result.CreatedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO validation_results(id, trajectory_id, task_id, step_id, validator_type, status, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)`, result.ID, result.TrajectoryID, result.TaskID, result.StepID, result.Validator, result.Status, created); err != nil {
		tx.Rollback()
		return err
	}
	if err := appendEvent(ctx, tx, result.TrajectoryID, "validation_result", map[string]any{"validation_id": result.ID, "validator": result.Validator, "status": result.Status}, created); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r SQLiteTelemetryRepository) Complete(ctx context.Context, trajectoryID string, completedAt time.Time) error {
	return r.finish(ctx, trajectoryID, "COMPLETED", "trajectory_completed", "", completedAt)
}

func (r SQLiteTelemetryRepository) Fail(ctx context.Context, trajectoryID, reason string, completedAt time.Time) error {
	return r.finish(ctx, trajectoryID, "FAILED", "trajectory_failed", reason, completedAt)
}

func (r SQLiteTelemetryRepository) CostReport(ctx context.Context, filter domain.ReportFilter) (domain.CostReport, error) {
	db, err := r.open()
	if err != nil {
		return domain.CostReport{}, err
	}
	defer db.Close()
	where, args := reportWhere(filter)
	query := `SELECT COUNT(*), COALESCE(SUM(status = 'COMPLETED'), 0), COALESCE(SUM(status = 'FAILED'), 0), COALESCE(SUM(status = 'RUNNING'), 0) FROM trajectories t` + where
	var report domain.CostReport
	if err := db.QueryRowContext(ctx, query, args...).Scan(&report.Trajectories, &report.Completed, &report.Failed, &report.InProgress); err != nil {
		return domain.CostReport{}, err
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_calls m JOIN trajectories t ON t.id = m.trajectory_id`+where, args...).Scan(&report.ModelCalls); err != nil {
		return domain.CostReport{}, err
	}
	var known sql.NullFloat64
	var withCost int
	if err := db.QueryRowContext(ctx, `SELECT SUM(m.actual_cost_usd), COUNT(m.actual_cost_usd) FROM model_calls m JOIN trajectories t ON t.id = m.trajectory_id`+where, args...).Scan(&known, &withCost); err != nil {
		return domain.CostReport{}, err
	}
	if known.Valid && withCost == report.ModelCalls {
		report.KnownCostUSD = &known.Float64
	}
	return report, nil
}

func (r SQLiteTelemetryRepository) TaskDistribution(ctx context.Context, filter domain.ReportFilter) ([]domain.TaskDistribution, error) {
	db, err := r.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	where, args := reportWhere(filter)
	rows, err := db.QueryContext(ctx, `SELECT k.task_family, COUNT(*) FROM tasks k JOIN trajectories t ON t.id = k.trajectory_id`+where+` GROUP BY k.task_family ORDER BY COUNT(*) DESC, k.task_family`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.TaskDistribution
	for rows.Next() {
		var entry domain.TaskDistribution
		if err := rows.Scan(&entry.TaskFamily, &entry.Count); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (r SQLiteTelemetryRepository) open() (*sql.DB, error) {
	db, err := sql.Open("sqlite", r.path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func startRun(ctx context.Context, tx *sql.Tx, run domain.BaselineRun) error {
	created := timestamp(run.StartedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions(id, status, active_execution_profile_id, policy_version, created_at, updated_at) VALUES(?, 'ACTIVE', ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET status = 'ACTIVE', active_execution_profile_id = excluded.active_execution_profile_id, policy_version = excluded.policy_version, updated_at = excluded.updated_at`, run.SessionID, run.ProfileID, run.PolicyVersion, created, created); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO trajectories(id, session_id, status, prompt_hash, active_policy_version, selected_execution_profile_id, command_budget_usd, started_at) VALUES(?, ?, 'RUNNING', ?, ?, ?, ?, ?)`, run.TrajectoryID, run.SessionID, run.PromptHash, run.PolicyVersion, run.ProfileID, run.CommandBudgetUS, created); err != nil {
		return fmt.Errorf("save trajectory: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tasks(id, trajectory_id, status, task_family, difficulty, created_at) VALUES(?, ?, 'RUNNING', 'unknown', 0, ?)`, run.TaskID, run.TrajectoryID, created); err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO steps(id, task_id, sequence_no, step_type, status, execution_profile_id, started_at) VALUES(?, ?, 1, 'baseline', 'RUNNING', ?, ?)`, run.StepID, run.TaskID, run.ProfileID, created); err != nil {
		return fmt.Errorf("save step: %w", err)
	}
	return appendEvent(ctx, tx, run.TrajectoryID, "trajectory_started", map[string]any{"profile_id": run.ProfileID, "policy_version": run.PolicyVersion}, created)
}

func (r SQLiteTelemetryRepository) finish(ctx context.Context, trajectoryID, status, eventType, reason string, completedAt time.Time) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	finished := timestamp(completedAt)
	result, err := tx.ExecContext(ctx, `UPDATE trajectories SET status = ?, completed_at = ? WHERE id = ? AND status = 'RUNNING'`, status, finished, trajectoryID)
	if err != nil {
		tx.Rollback()
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return err
	}
	if count != 1 {
		tx.Rollback()
		return fmt.Errorf("trajectory %s is not running", trajectoryID)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tasks SET status = ?, completed_at = ? WHERE trajectory_id = ? AND status = 'RUNNING'`, status, finished, trajectoryID); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE steps SET status = ?, completed_at = ? WHERE task_id IN (SELECT id FROM tasks WHERE trajectory_id = ?) AND status = 'RUNNING'`, status, finished, trajectoryID); err != nil {
		tx.Rollback()
		return err
	}
	payload := map[string]any{}
	if reason != "" {
		payload["reason"] = reason
	}
	if err := appendEvent(ctx, tx, trajectoryID, eventType, payload, finished); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func appendEvent(ctx context.Context, tx *sql.Tx, trajectoryID, eventType string, payload map[string]any, createdAt string) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO trajectory_events(trajectory_id, event_type, event_version, payload_json, created_at) VALUES(?, ?, 1, ?, ?)`, trajectoryID, eventType, string(encoded), createdAt)
	return err
}

func optional[T any](payload map[string]any, name string, value *T) {
	if value != nil {
		payload[name] = *value
	}
}

func reportWhere(filter domain.ReportFilter) (string, []any) {
	where := " WHERE t.started_at >= ?"
	args := []any{timestamp(filter.Since)}
	if filter.SessionID != "" {
		where += " AND t.session_id = ?"
		args = append(args, filter.SessionID)
	}
	if filter.ProfileID != "" {
		where += " AND t.selected_execution_profile_id = ?"
		args = append(args, filter.ProfileID)
	}
	if filter.Outcome != "" {
		where += " AND t.status = ?"
		args = append(args, filter.Outcome)
	}
	return where, args
}

func timestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
