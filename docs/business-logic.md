# GrandetAgent Business Logic

## 1. Purpose

This document defines the application and domain behavior of GrandetAgent.

The architecture document explains what the system is. This document explains how the system behaves when commands are executed and how business components coordinate without leaking rules into CLI handlers, database repositories, or provider adapters.

## 2. Layering

```text
CLI Layer
  Parses commands and renders results.

Application Layer
  Implements use cases and transaction boundaries.

Domain Layer
  Owns routing, cost, validation, feedback, policy, and state-transition rules.

Infrastructure Layer
  Implements providers, SQLite repositories, files, tokenizers, validators, and clocks.
```

Dependency direction:

```text
CLI -> Application -> Domain <- Infrastructure
```

The domain layer must not import CLI packages, provider SDKs, SQL drivers, or filesystem-specific code.

## 3. Main Application Services

### 3.1 InitializeWorkspaceService

Used by:

```bash
grandet init
```

Responsibilities:

1. Resolve the GrandetAgent home directory.
2. Create directory layout.
3. Create default configuration and initial policy.
4. Initialize SQLite schema.
5. Record schema and configuration versions.
6. Preserve existing files unless `--force` is specified.

The service does not validate provider credentials or call paid APIs.

### 3.2 ExecuteTrajectoryService

Used by:

```bash
grandet run "..."
```

This is the primary use case. It coordinates session loading, classification, routing, execution, validation, escalation, cost accounting, and result persistence.

### 3.3 RecordFeedbackService

Used by:

```bash
grandet accept <trajectory-id>
grandet reject <trajectory-id>
grandet rate <trajectory-id>
```

Responsibilities:

- validate that the trajectory exists
- append an immutable feedback event
- recalculate relevant evidence windows
- generate a policy draft when adjustment thresholds are crossed
- never directly modify the active policy

### 3.4 ReplayTrajectoryService

Used by:

```bash
grandet task replay <trajectory-id>
grandet eval shadow --trajectory <trajectory-id>
```

Responsibilities:

- load the historical state snapshot
- substitute policy or candidate profiles
- preserve the original trajectory
- execute a new shadow trajectory
- compare outcomes and costs

### 3.5 ManagePolicyService

Used by:

```bash
grandet policy validate
grandet policy activate
grandet policy rollback
grandet policy diff
```

Responsibilities:

- parse and validate policy YAML
- run Golden Set regression
- evaluate routing overhead and cost estimates
- activate or rollback policy versions
- preserve activation history

### 3.6 ManageModelService

Used by:

```bash
grandet model sync
grandet model profile
grandet model smoke-test
grandet model promote
grandet model quarantine
```

Responsibilities:

- discover model metadata
- create execution profiles
- update prices and capabilities
- run health and smoke tests
- control free-model lifecycle

### 3.7 AnalyzeService

Used by:

```bash
grandet analyze cost
grandet analyze savings
grandet analyze routing
grandet analyze rejects
```

It reads persisted facts and produces reports. It does not change routing behavior.

## 4. Execute Trajectory Use Case

### 4.1 Input

```go
type ExecuteTrajectoryCommand struct {
    SessionID       string
    Prompt          string
    ContextFiles    []string
    ExplicitProfile string
    MaxCostUSD      *float64
    Trace           bool
}
```

An explicit profile is a user override. It bypasses normal profile ranking but does not bypass safety constraints, cost accounting, or validation.

### 4.2 Transaction boundary

A trajectory must be created before any potentially paid work occurs.

The initial transaction stores:

- trajectory ID
- session ID
- prompt hash
- active policy version
- start time
- command-level budget

Every paid or state-changing action is appended as an event.

### 4.3 Main flow

```text
1. Load or create session.
2. Create trajectory in PLANNING state.
3. Open trajectory cost ledger.
4. Load active routing policy.
5. Build context plan.
6. Analyze task family and difficulty.
7. Capture routing state snapshot.
8. Build viable execution-profile candidates.
9. Estimate expected trajectory cost for each candidate.
10. Select the lowest-cost acceptable candidate.
11. Persist routing decision before execution.
12. Execute selected profile.
13. Persist call usage and normalized provider outcome.
14. Validate result.
15. Repair, expand context, escalate, or fall back when required.
16. Complete technical trajectory state.
17. Present result to user.
18. Later attach explicit or implicit feedback.
```

### 4.4 Trajectory state machine

```text
CREATED
  -> PLANNING
  -> ROUTED
  -> EXECUTING
  -> VALIDATING
  -> REPAIRING
  -> ESCALATING
  -> VALIDATED_SUCCESS
  -> ACCEPTED

Failure terminals:
  REJECTED
  BUDGET_EXHAUSTED
  FATAL_FAILURE
  ABANDONED
```

`VALIDATED_SUCCESS` is a technical state. `ACCEPTED` is a user outcome.

A trajectory can be technically valid and later rejected by the user.

## 5. Context Planning Logic

### 5.1 Goals

The Context Planner minimizes unnecessary input tokens while preserving enough evidence to complete the task.

### 5.2 Steps

1. Read explicit prompt and context references.
2. Normalize line endings and encoding.
3. Deduplicate exact content blocks.
4. Detect stable reusable prefixes.
5. Estimate tokens by provider-compatible tokenizer where available.
6. Classify context size.
7. Estimate replay cost for switching away from the session-affine profile.
8. Produce profile-specific context feasibility results.

### 5.3 Output

```go
type ContextPlan struct {
    PromptTokens             int
    ContextTokens            int
    ReusablePrefixTokens     int
    EstimatedReplayTokens    int
    ContextSizeClass         string
    RequiredContextWindow    int
    MissingInformation       []string
}
```

### 5.4 Tradeoff

The first version may only deduplicate and trim explicit history. Semantic chunk retrieval is deferred because retrieval quality introduces its own evaluation problem.

## 6. Task Analysis Logic

### 6.1 Layered analyzer

The analyzer uses the cheapest sufficient stage.

#### L0: hard constraints

Examples:

- context window requirement
- tool-call declaration
- image or audio input
- required JSON schema
- explicit high-risk domain

L0 can exclude profiles but does not need to assign a precise task family.

#### L1: local rules

Examples:

- file extension
- CLI flags
- known command prefixes
- keywords such as `debug`, `review`, `summarize`
- repository or Kubernetes context detection

#### L2: local semantic classifier

Embedding similarity or a lightweight local classifier produces:

- task family probabilities
- difficulty distribution
- domain probabilities

#### L3: optional model classifier

Used only when:

```text
classification_confidence < threshold
and expected_saving > classifier_cost + risk_margin
```

### 6.2 Confidence fallback

When evidence is sparse or ambiguous, GrandetAgent chooses a coarser task family or raises the success threshold. It must not fabricate precision.

## 7. Candidate Building Logic

### 7.1 Candidate source

Candidates come from enabled Model Execution Profiles, not directly from provider model lists.

### 7.2 Hard filtering

A candidate is removed when any condition is true:

- provider disabled or unhealthy
- model disabled or quarantined
- context window too small
- required tool or modality unsupported
- safety/domain restriction violated
- minimum output limit impossible
- task or daily budget exceeded
- free model not admitted for the task class

Every removal records a reason.

### 7.3 Session affinity

For the currently affine profile, Candidate Builder records:

```text
affinity_bonus
estimated_cache_reuse
zero_switch_penalty
```

For alternative profiles, it records:

```text
switch_penalty
context_replay_cost
continuity_risk
```

### 7.4 Recovery chains

Candidate Builder produces:

```go
type CandidateSet struct {
    PrimaryCandidates          []ExecutionCandidate
    QualityEscalationChain     []ExecutionCandidate
    ReliabilityFallbackChain  []ExecutionCandidate
}
```

The quality chain is sorted by increasing expected success quality and cost.

The reliability chain prefers equivalent capability through another provider before changing model behavior.

## 8. Cost Estimation Logic

### 8.1 Raw call estimate

```text
raw_call_cost
  = input_tokens * input_rate
  + cached_input_tokens * cached_input_rate
  + output_tokens * output_rate
  + reasoning_tokens * reasoning_rate
```

### 8.2 Expected trajectory estimate

```text
expected_trajectory_cost
  = routing_cost
  + raw_call_cost
  + expected_tool_cost
  + expected_validation_cost
  + retry_probability * retry_cost
  + repair_probability * repair_cost
  + quality_escalation_probability * escalation_cost
  + reliability_failure_probability * fallback_cost
  + model_switch_penalty
  + context_replay_cost
```

### 8.3 Sparse evidence behavior

When local samples are insufficient:

1. Use public benchmark and provider metadata as weak priors.
2. Increase uncertainty margin.
3. Prefer reversible, low-risk choices.
4. Avoid promoting a free model directly into high-risk work.

### 8.4 Cost ledger

The ledger is append-only during execution.

Each entry contains:

```text
category
estimated amount
actual amount
currency
provider usage source
associated step or call
```

## 9. Policy Evaluation Logic

### 9.1 Constraint phase

A candidate must pass:

```text
safety
capability
context
budget
minimum expected success
maximum latency
free-model admission
```

### 9.2 Ranking phase

Candidates are sorted by expected trajectory cost.

Ties or near-ties are resolved by:

1. lower uncertainty
2. lower fallback tax
3. session affinity
4. lower latency
5. stronger local accept rate

### 9.3 Switching rule

A profile switch is allowed only when:

```text
expected_cost_current
  - expected_cost_alternative
  > switch_penalty + context_replay_cost + uncertainty_margin
```

### 9.4 Reasoning-mode rule

Before switching model families, the policy evaluates whether a cheaper reasoning profile of the current model can satisfy the task.

Example order:

```text
current model / no-thinking
current model / low reasoning
cheap alternative model
current model / high reasoning
strong alternative model
```

The actual order is task- and evidence-dependent.

### 9.5 User override

A user-selected profile is honored when valid. The trace records that the route was manually overridden and excludes it from some automatic policy-quality judgments.

## 10. Execution and Provider Outcome Logic

Provider adapters normalize results into:

```go
type CallOutcome struct {
    Status           CallStatus
    Text             string
    ToolCalls        []ToolCall
    InputTokens      int
    CachedTokens     int
    OutputTokens     int
    ReasoningTokens  int
    TTFT             time.Duration
    TotalLatency     time.Duration
    ProviderError    *NormalizedProviderError
}
```

Normalized reliability failures:

```text
timeout
rate_limit
network_error
provider_5xx
model_unavailable
context_limit
invalid_provider_response
```

These failures trigger reliability fallback, not quality penalty.

## 11. Validation and Repair Logic

### 11.1 Validation plan

The task profile chooses validators.

Examples:

```text
extraction -> JSON parser + schema
configuration -> YAML parser
code patch -> formatter + compiler + tests
shell plan -> static risk validator
knowledge answer -> acceptance criteria + optional judge
```

### 11.2 Result classes

```text
PASS
REPAIRABLE_FAILURE
INSUFFICIENT_CONTEXT
QUALITY_FAILURE
SAFETY_FAILURE
```

### 11.3 Repair order

```text
1. deterministic repair
2. same-profile constrained retry
3. context expansion
4. quality escalation
5. fail safely
```

A formatting problem should not immediately invoke a premium model.

### 11.4 Judge use

A live-path judge is permitted only when:

- deterministic validators cannot decide
- judge cost is below policy budget
- expected escalation savings justify the judge

Offline evaluation may use stronger or multiple judges.

## 12. Escalation Logic

### 12.1 Quality escalation

Triggered by:

- failed acceptance criteria
- unrepairable schema result
- incorrect tool plan
- low validated quality
- structured `no_answer` after context expansion is exhausted

Quality escalation may replay part of the context. That replay is charged to the current trajectory and the original candidate's fallback tax.

### 12.2 Reliability fallback

Triggered by normalized provider failure.

Order:

1. equivalent profile through another provider
2. equivalent model with similar execution mode
3. next cheapest acceptable profile
4. stop when reliability fallback depth or budget is exhausted

### 12.3 Budget fallback

When remaining budget cannot fund the next action:

1. try a cheaper valid profile
2. reduce optional output or reasoning limits when policy allows
3. return a structured budget-exhausted result

GrandetAgent never silently exceed the user's configured hard budget.

## 13. Feedback Logic

### 13.1 Explicit feedback

```text
accept
reject with reason
rating
manual model selection
manual replay
```

### 13.2 Implicit feedback

GrandetAgent may identify a possible re-ask when:

- a new task occurs soon after the prior result
- semantic similarity is high
- wording contains correction or dissatisfaction signals
- the previous trajectory was not explicitly accepted

Possible re-ask remains a weak signal until confirmed by repeated patterns.

### 13.3 Evidence windows

Maintain at least:

```text
recent window
stable window
lifetime totals
```

Example:

```text
recent: 7 days
stable: 30 days
```

Recent evidence adapts to drift. Stable evidence prevents one bad event from causing overreaction.

### 13.4 Update behavior

Feedback updates:

- model execution profile statistics
- task-family and difficulty tolerance
- re-ask and reject rates
- expected escalation probability
- expected accepted-trajectory cost

It does not immediately change the active policy.

## 14. Policy Draft Generation

A new draft may be generated when:

- minimum evidence count is met
- a cheaper profile repeatedly succeeds
- current preferred profile has persistent rejection
- routing overhead exceeds budget
- provider reliability changes materially
- a new stable free model is admitted

A draft contains:

```text
base policy version
changed rules
supporting evidence
expected cost effect
expected quality effect
uncertainty
rollback target
```

## 15. Policy Validation and Activation

### 15.1 Static validation

- YAML schema
- referenced providers and profiles exist
- no impossible fallback loops
- budget rules are internally consistent
- all red lines are present

### 15.2 Offline validation

- run Golden Set
- run safety and regression sets
- compare trajectory cost
- measure routing overhead
- calculate quality-retention metrics

### 15.3 Activation

The user explicitly activates a validated policy in the first version.

Activation persists:

- previous version
- new version
- validation report
- activation time
- rollback target

## 16. Golden Set Logic

### 16.1 Case structure

```yaml
id: k8s-debug-001
task_family: debugging
difficulty: 3
domain: kubernetes
prompt: ...
acceptance_criteria:
  - distinguishes Pod networking from Service networking
  - checks kube-proxy state
  - avoids destructive remediation
validators:
  - required_concepts
  - safety
  - pairwise_judge
```

### 16.2 Initial scope

A practical initial set:

```text
10 general cases
10 coding cases
10 Kubernetes/SRE cases
```

Quality matters more than volume.

### 16.3 Evaluation outputs

- pass rate
- quality retention
- PGR/APGR/CPT where strong and weak baselines exist
- cost per successful trajectory
- routing overhead
- escalation and fallback rates

## 17. Shadow Replay Logic

### 17.1 Input snapshot

A replay uses the original:

```text
prompt and context references
task profile
session state snapshot
candidate metadata
provider prices at the time or current prices, explicitly selected
```

### 17.2 Replay modes

```text
policy replay
profile comparison
provider comparison
reasoning-mode comparison
```

### 17.3 Result

```text
quality delta
cost delta
latency delta
fallback-tax delta
routing recommendation
confidence and sample count
```

Shadow results generate recommendations or policy drafts. They do not silently activate behavior.

## 18. Model Discovery and Free-Model Governance

### 18.1 Discovery

New models enter `DISCOVERED` with low confidence.

### 18.2 Smoke test

Smoke tests cover:

- endpoint availability
- basic instruction following
- structured output where claimed
- tool call where claimed
- context acceptance
- usage metadata availability

### 18.3 Promotion

Promotion requires:

- successful smoke tests
- acceptable reliability
- at least one task-family profile or Golden Set result
- no policy restriction violation

### 18.4 Degradation and quarantine

Triggers:

- repeated rate limits
- repeated timeout
- capability mismatch
- reject or escalation rate regression
- provider removal
- price change that destroys value

## 19. Analysis Logic

### 19.1 Cost report

Reports:

- raw call cost
- total trajectory cost
- successful and accepted trajectory cost
- routing overhead
- fallback tax
- context replay tax
- rejected work

### 19.2 Savings report

Compares actual trajectories with a configured baseline.

The report distinguishes:

```text
measured baseline
estimated baseline
```

### 19.3 Routing report

Shows:

- selected and rejected candidates
- selection reasons
- session-switch decisions
- profile performance by family and difficulty
- policy-version comparisons

## 20. Domain Interfaces

Representative interfaces:

```go
type TrajectoryRepository interface {
    Create(ctx context.Context, t Trajectory) error
    AppendEvent(ctx context.Context, id string, event DomainEvent) error
    Get(ctx context.Context, id string) (Trajectory, error)
}

type PolicyRepository interface {
    GetActive(ctx context.Context) (RoutingPolicy, error)
    SaveDraft(ctx context.Context, policy RoutingPolicy) error
    Activate(ctx context.Context, version string) error
    Rollback(ctx context.Context, version string) error
}

type ExecutionProfileRepository interface {
    ListEligible(ctx context.Context) ([]ModelExecutionProfile, error)
    UpdateEvidence(ctx context.Context, evidence ProfileEvidence) error
}

type ProviderGateway interface {
    Execute(ctx context.Context, req ProviderRequest) (CallOutcome, error)
}
```

## 21. Invariants

The business layer must enforce:

1. No paid call exists outside a trajectory.
2. Every routing decision references a policy version.
3. Every model call references an execution profile.
4. Actual cost entries are append-only.
5. Reliability failures do not count as quality failures.
6. User feedback is immutable and additive.
7. Active policy changes are versioned and reversible.
8. Hard budgets are never silently exceeded.
9. A quarantined model cannot be selected for normal execution.
10. Shadow execution never overwrites the original trajectory.

## 22. Error Handling Philosophy

Errors are divided into:

```text
user input error
configuration error
provider reliability error
validation error
policy error
storage error
budget error
```

The application layer converts domain and infrastructure errors into stable CLI error codes and clear remediation messages.

A provider failure should not corrupt the trajectory. A storage failure after a paid call must be surfaced prominently because losing cost evidence violates auditability.
