package domain

import "context"

// ModelCapabilities is provider-neutral so routing never depends on an
// upstream provider's field names.
type ModelCapabilities struct {
	ToolCalling bool
	JSONOutput  bool
	Vision      bool
}

type ModelPrice struct {
	InputPerMillion       *float64
	OutputPerMillion      *float64
	ReasoningPerMillion   *float64
	CachedInputPerMillion *float64
	EffectiveFrom         string
	Source                string
}

type ProviderModel struct {
	ID             string
	ContextWindow  *int
	Capabilities   ModelCapabilities
	IsFree         bool
	Price          *ModelPrice
	SourceMetadata string
}

// ModelExecutionProfile is the only model-selection value a router needs.
type ModelExecutionProfile struct {
	ID              string
	Provider        string
	Model           string
	ReasoningMode   string
	ReasoningEffort string
	MaxOutputTokens int
	Capabilities    ModelCapabilities
	RetryPolicy     string
	QualityTier     string
	Enabled         bool
	LifecycleState  string
	IsFree          bool
	PriceKnown      bool
}

func (p ModelExecutionProfile) EligibleForAutomaticRouting(allowUnknownPaid bool) bool {
	return p.Enabled && p.LifecycleState != "QUARANTINED" && p.LifecycleState != "DISABLED" && (p.IsFree || p.PriceKnown || allowUnknownPaid)
}

// ExecutionProfileReader intentionally exposes profiles, not raw provider models.
type ExecutionProfileReader interface {
	EligibleExecutionProfiles(context.Context, bool) ([]ModelExecutionProfile, error)
}
