package domain

import "time"

// BaselineRun is the durable record made before any provider work starts.
type BaselineRun struct {
	SessionID       string
	TrajectoryID    string
	TaskID          string
	StepID          string
	ProfileID       string
	PolicyVersion   string
	PromptHash      string
	CommandBudgetUS *float64
	StartedAt       time.Time
}

type ReportFilter struct {
	Since     time.Time
	SessionID string
	ProfileID string
	Outcome   string
}

type CostReport struct {
	Trajectories int
	Completed    int
	Failed       int
	InProgress   int
	ModelCalls   int
	KnownCostUSD *float64
}

type TaskDistribution struct {
	TaskFamily string
	Count      int
}

type Event struct {
	TrajectoryID string
	Type         string
	Payload      map[string]any
	CreatedAt    time.Time
}

type ModelCall struct {
	ID                  string
	TrajectoryID        string
	TaskID              string
	StepID              string
	ProfileID           string
	ProviderRequestID   *string
	InputTokens         *int
	OutputTokens        *int
	ReasoningTokens     *int
	TTFTMilliseconds    *int
	LatencyMilliseconds *int
	ActualCostUSD       *float64
	ErrorType           *string
	StartedAt           time.Time
	CompletedAt         time.Time
}

type ToolCall struct {
	ID           string
	TrajectoryID string
	TaskID       string
	StepID       string
	Name         string
	Outcome      string
	LatencyMS    *int
	ErrorType    *string
	CreatedAt    time.Time
}

type ValidationResult struct {
	ID           string
	TrajectoryID string
	TaskID       string
	StepID       *string
	Validator    string
	Status       string
	CreatedAt    time.Time
}
