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
	StartModelCall(context.Context, domain.ModelCall) error
	CompleteModelCall(context.Context, domain.ModelCall) error
	Complete(context.Context, string, time.Time) error
	Fail(context.Context, string, string, time.Time) error
	CostReport(context.Context, domain.ReportFilter) (domain.CostReport, error)
	TaskDistribution(context.Context, domain.ReportFilter) ([]domain.TaskDistribution, error)
}

type ProviderExecutor interface {
	Execute(context.Context, string) (domain.ProviderResult, error)
}

type ReportFilter = domain.ReportFilter

type RunOptions struct {
	SessionID       string
	ProfileID       string
	TaskFamily      string
	Prompt          string
	CommandBudgetUS *float64
}

type BaselineRunner struct {
	repository TelemetryRepository
	clock      domain.Clock
	ids        domain.IDGenerator
	executor   ProviderExecutor
}

func NewBaselineRunner(repository TelemetryRepository, clock domain.Clock, ids domain.IDGenerator, executor ProviderExecutor) BaselineRunner {
	return BaselineRunner{repository: repository, clock: clock, ids: ids, executor: executor}
}

// Start persists the complete partial trajectory before a caller can make a paid call.
func (r BaselineRunner) Start(ctx context.Context, options RunOptions) (domain.BaselineRun, error) {
	if options.ProfileID == "" {
		return domain.BaselineRun{}, fmt.Errorf("profile is required")
	}
	if options.Prompt == "" {
		return domain.BaselineRun{}, fmt.Errorf("prompt is required")
	}
	if options.TaskFamily != "" && !domain.IsTaskFamily(domain.TaskFamily(options.TaskFamily)) {
		return domain.BaselineRun{}, fmt.Errorf("unknown task family %q", options.TaskFamily)
	}
	profile := domain.ClassifyTask(domain.TaskInput{Prompt: options.Prompt, TaskFamilyOverride: domain.TaskFamily(options.TaskFamily)})
	sessionID := options.SessionID
	if sessionID == "" {
		sessionID = r.ids.New()
	}
	now := r.clock.Now().UTC()
	run := domain.BaselineRun{
		SessionID: sessionID, TrajectoryID: r.ids.New(), TaskID: r.ids.New(), StepID: r.ids.New(),
		ProfileID: options.ProfileID, TaskFamily: string(profile.TaskFamily.Value), TaskProfile: profile, PolicyVersion: "stingy-v1", PromptHash: promptHash(options.Prompt),
		CommandBudgetUS: options.CommandBudgetUS, StartedAt: now,
	}
	if err := r.repository.Start(ctx, run); err != nil {
		return domain.BaselineRun{}, fmt.Errorf("persist trajectory before execution: %w", err)
	}
	return run, nil
}

// Execute records the provider call around the real fixed-profile request.
func (r BaselineRunner) Execute(ctx context.Context, options RunOptions) (domain.BaselineRun, domain.ProviderResult, error) {
	if r.executor == nil {
		return domain.BaselineRun{}, domain.ProviderResult{}, fmt.Errorf("provider executor is required")
	}
	run, err := r.Start(ctx, options)
	if err != nil {
		return domain.BaselineRun{}, domain.ProviderResult{}, err
	}
	started := r.clock.Now().UTC()
	call := domain.ModelCall{ID: r.ids.New(), TrajectoryID: run.TrajectoryID, TaskID: run.TaskID, StepID: run.StepID, ProfileID: run.ProfileID, StartedAt: started}
	if err := r.repository.StartModelCall(ctx, call); err != nil {
		_ = r.Fail(ctx, run.TrajectoryID, "record_model_call")
		return run, domain.ProviderResult{}, fmt.Errorf("record provider call before execution: %w", err)
	}
	result, executeErr := r.executor.Execute(ctx, options.Prompt)
	completed := r.clock.Now().UTC()
	call.CompletedAt = completed
	call.ProviderRequestID = result.ProviderRequestID
	call.InputTokens = result.InputTokens
	call.OutputTokens = result.OutputTokens
	call.ReasoningTokens = result.ReasoningTokens
	if executeErr != nil {
		errorType := "provider"
		call.ErrorType = &errorType
	} else {
		call.ActualCostUSD = result.ActualCostUSD
	}
	call.LatencyMilliseconds = milliseconds(completed.Sub(started))
	if err := r.repository.CompleteModelCall(ctx, call); err != nil {
		return run, result, fmt.Errorf("record provider result: %w", err)
	}
	if executeErr != nil {
		if err := r.Fail(ctx, run.TrajectoryID, "provider"); err != nil {
			return run, result, err
		}
		return run, result, fmt.Errorf("execute provider: %w", executeErr)
	}
	if err := r.Complete(ctx, run.TrajectoryID); err != nil {
		return run, result, err
	}
	return run, result, nil
}

func milliseconds(value time.Duration) *int {
	result := int(value.Milliseconds())
	return &result
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
