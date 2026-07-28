package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type TelemetryRepository interface {
	Start(context.Context, domain.BaselineRun) error
	Complete(context.Context, string, time.Time) error
	Fail(context.Context, string, string, time.Time) error
	CostReport(context.Context, domain.ReportFilter) (domain.CostReport, error)
	TaskDistribution(context.Context, domain.ReportFilter) ([]domain.TaskDistribution, error)
}

type RunOptions struct {
	SessionID       string
	ProfileID       string
	Prompt          string
	CommandBudgetUS *float64
}

type BaselineRunner struct {
	repository TelemetryRepository
	clock      domain.Clock
	ids        domain.IDGenerator
}

func NewBaselineRunner(repository TelemetryRepository, clock domain.Clock, ids domain.IDGenerator) BaselineRunner {
	return BaselineRunner{repository: repository, clock: clock, ids: ids}
}

// Start persists the complete partial trajectory before a caller can make a paid call.
func (r BaselineRunner) Start(ctx context.Context, options RunOptions) (domain.BaselineRun, error) {
	if options.ProfileID == "" {
		return domain.BaselineRun{}, fmt.Errorf("profile is required")
	}
	if options.Prompt == "" {
		return domain.BaselineRun{}, fmt.Errorf("prompt is required")
	}
	sessionID := options.SessionID
	if sessionID == "" {
		sessionID = r.ids.New()
	}
	now := r.clock.Now().UTC()
	run := domain.BaselineRun{
		SessionID: sessionID, TrajectoryID: r.ids.New(), TaskID: r.ids.New(), StepID: r.ids.New(),
		ProfileID: options.ProfileID, PolicyVersion: "stingy-v1", PromptHash: promptHash(options.Prompt),
		CommandBudgetUS: options.CommandBudgetUS, StartedAt: now,
	}
	if err := r.repository.Start(ctx, run); err != nil {
		return domain.BaselineRun{}, fmt.Errorf("persist trajectory before execution: %w", err)
	}
	return run, nil
}

func (r BaselineRunner) Complete(ctx context.Context, trajectoryID string) error {
	if err := r.repository.Complete(ctx, trajectoryID, r.clock.Now().UTC()); err != nil {
		return fmt.Errorf("complete trajectory: %w", err)
	}
	return nil
}

func (r BaselineRunner) Fail(ctx context.Context, trajectoryID, reason string) error {
	if err := r.repository.Fail(ctx, trajectoryID, reason, r.clock.Now().UTC()); err != nil {
		return fmt.Errorf("fail trajectory: %w", err)
	}
	return nil
}

func promptHash(prompt string) string {
	digest := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(digest[:])
}
