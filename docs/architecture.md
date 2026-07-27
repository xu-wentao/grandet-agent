# GrandetAgent Architecture

## 1. Project Definition

GrandetAgent is a local-first, cost-optimizing Agent CLI.

It does not try to become a general workflow platform or a full AI gateway. Its responsibility is narrower:

> For each Agent trajectory, choose and coordinate the cheapest execution profiles that are still likely to produce an acceptable result for the current user.

The routing target is not only a model name. It is a complete **Model Execution Profile**:

```text
provider + model + reasoning mode + token limits + tool capability + cache state + retry policy
```

GrandetAgent runs locally, stores all decisions and feedback locally, and exposes its behavior through CLI commands and versioned YAML policies.

## Package Boundaries and Conventions

`internal/domain` contains pure business contracts and may import only the Go standard library. `internal/application` owns use cases and may depend on domain contracts, never CLI or infrastructure. `internal/infrastructure` supplies adapters for those contracts. `internal/cli` is the composition root: it parses input, wires adapters, and calls application use cases.

Package names describe the responsibility (`domain`, `application`, `infrastructure`, `cli`), not a transport or vendor. Wrap an operation failure at the boundary that adds useful context, using `%w` (for example, `fmt.Errorf("migrate database: %w", err)`); do not wrap an error again without adding context.

Fast tests run with `go test ./...`. SQLite adapter tests are isolated behind the `integration` build tag and run with `go test -tags=integration ./internal/infrastructure`. Dependency conformance tests run in the fast suite and reject forbidden imports.

`internal/testkit` provides configurable clock, ID generator, provider, repository, and validator doubles. Keep these test-only and add a production port only when a use case needs one.

## 2. Design Philosophy

### 2.1 Stinginess is rational optimization, not cheapest-call obsession

The cheapest individual call is not necessarily the cheapest successful task.

A cheap model may create hidden cost through:

- invalid output
- retries
- tool failures
- context replay
- judge calls
- quality escalation
- user rejection and re-execution

GrandetAgent therefore optimizes:

```text
cost_per_successful_trajectory
cost_per_accepted_trajectory
```

rather than only:

```text
input_price_per_token
output_price_per_token
```

### 2.2 Price is first, success is a constraint

The strategy is not a configurable collection of personalities such as `balanced`, `fast`, and `quality`.

GrandetAgent has one philosophy:

```text
minimize trajectory cost
subject to acceptable success probability and explicit safety constraints
```

The acceptable threshold is user-specific and task-specific. It is learned conservatively from explicit and implicit feedback.

### 2.3 Spend intelligence only when its expected value is positive

Routing itself has a cost.

A router that invokes another expensive LLM for every request may spend more than it saves. Every routing stage therefore has a budget:

```text
expected_routing_value
  = expected_saving
  - routing_cost
  - expected_misroute_cost
```

A more expensive classifier is used only when `expected_routing_value > 0`.

### 2.4 Deterministic before probabilistic

Before paying for a stronger model, GrandetAgent should attempt cheaper deterministic work:

- parse JSON or YAML
- validate a schema
- run a formatter
- compile or lint code
- validate tool arguments
- detect missing context

A model upgrade is justified only after cheap validation and repair paths are exhausted or unsafe.

### 2.5 Local evidence defeats generic benchmark prestige

Public benchmarks are useful for cold start, but they cannot represent the user's real tasks, provider reliability, tool behavior, context shape, or tolerance.

Evidence priority:

```text
explicit user feedback
  > validated local trajectory outcome
  > implicit user behavior
  > local shadow replay
  > local Golden Set
  > public benchmark
  > provider marketing metadata
```

### 2.6 Continuity is an economic asset

Switching models inside a long session may destroy provider-side cache value and require large context replay.

GrandetAgent therefore treats session affinity, warm prefixes, reasoning continuity, and context replay as economic inputs rather than incidental implementation details.

### 2.7 Learning must be reversible

User feedback may be noisy and task distributions may drift. GrandetAgent never silently mutates an opaque global policy.

All learned changes produce a versioned policy candidate that can be:

```text
validated -> activated -> degraded -> frozen -> rolled back
```

## 3. Product Boundary

GrandetAgent owns:

- local provider and model configuration
- model discovery and profiling
- task-family and difficulty classification
- execution-profile selection
- session affinity and switch penalties
- trajectory execution and cost accounting
- deterministic validation
- quality escalation and reliability fallback
- user feedback learning
- Golden Set and shadow replay
- policy generation, validation, activation, and rollback
- CLI analysis and trace output

GrandetAgent does not initially own:

- visual workflow design
- enterprise multi-tenancy
- centralized gateway control plane
- hosted observability platform
- Kubernetes deployment
- distributed worker scheduling
- online traffic canary and A/B infrastructure

It can use existing infrastructure beneath it:

```text
GrandetAgent
  -> OpenRouter
  -> LiteLLM
  -> OpenAI-compatible APIs
  -> local Ollama / vLLM runtimes
```

## 4. Fundamental Domain Model

### 4.1 Session

A user-visible conversation or work context.

The session owns:

- model affinity
- reusable context prefix
- accumulated context size
- recent task families
- current policy version
- estimated cache state

### 4.2 Trajectory

The complete economic and quality unit.

A trajectory begins when the user submits a goal and ends when one of the following occurs:

```text
accepted
validated_success
rejected
abandoned
budget_exhausted
fatal_failure
```

A trajectory may contain several tasks, steps, model calls, tool calls, retries, repairs, and escalations.

### 4.3 Task

A semantically coherent objective inside a trajectory.

Examples:

- inspect repository structure
- diagnose a Kubernetes error
- produce a patch
- summarize a document
- validate generated JSON

### 4.4 Step

A single executable action inside a task.

A step may be:

- deterministic local operation
- model call
- tool call
- validator
- repair operation
- judge operation

### 4.5 Model Execution Profile

The actual unit selected by the router.

```yaml
id: openrouter-qwen-fast
provider: openrouter
model: qwen/example
reasoning:
  mode: disabled
max_output_tokens: 1200
temperature: 0.2
tool_calling: true
context_window: 131072
price_profile: standard
```

The same model may have several profiles:

```text
model/no-thinking
model/low-reasoning
model/high-reasoning
model/short-output
model/tool-enabled
```

This allows GrandetAgent to save money without switching model families unnecessarily.

### 4.6 Routing Policy

A versioned set of signals, constraints, ranking rules, escalation rules, and red lines.

A policy never directly contains user secrets. It references registered providers and profiles.

## 5. Task Taxonomy

Routing quality is modeled on two primary axes.

### 5.1 Task family

Initial families:

```text
general_qa
classification
extraction
documentation
summarization
code_generation
code_review
debugging
architecture_design
test_generation
error_recovery
data_analysis
tool_use_planning
kubernetes_troubleshooting
```

### 5.2 Difficulty

```text
1 trivial
2 simple
3 moderate
4 complex
5 expert
```

### 5.3 Additional dimensions

```text
domain
risk_level
seriousness
context_size
required_tools
required_modalities
verification_mode
latency_requirement
```

Example:

```json
{
  "task_family": "debugging",
  "difficulty": 3,
  "domain": "kubernetes",
  "risk_level": "medium",
  "seriousness": "production",
  "context_size": "large",
  "required_tools": ["file_read", "shell"],
  "verification_mode": "command_and_log_check"
}
```

Runtime profiles are aggregated primarily by:

```text
user + execution_profile + task_family + difficulty + domain
```

## 6. System Architecture

```text
CLI / Application Layer
  |
  v
Trajectory Service
  |
  +--> Session Manager
  +--> Context Planner
  +--> Task Analyzer
  +--> Candidate Builder
  +--> Policy Engine
  +--> Execution Coordinator
  +--> Validation Coordinator
  +--> Escalation Manager
  +--> Cost Ledger
  +--> Feedback Service
  +--> Evaluation Service
  |
  v
Domain Stores
  +--> Model Registry
  +--> Profile Store
  +--> Policy Store
  +--> Trajectory Store
  +--> Evaluation Store
  |
  v
Infrastructure Adapters
  +--> Provider adapters
  +--> Tokenizers
  +--> SQLite
  +--> File system
  +--> Deterministic validators
  +--> Local embedding/classifier runtime
```

## 7. Core Components

### 7.1 CLI Layer

The CLI is the first-version user interface.

It parses commands, renders output, and invokes application services. It must not contain routing or learning rules.

The foundation follows the same direction in code: `internal/cli` constructs application services, `internal/application` owns initialization orchestration and ports, `internal/domain` exposes time and ID ports, and `internal/infrastructure` provides filesystem, clock, and SQLite adapters. Dependencies point inward; domain imports neither filesystem nor SQL code.

### 7.2 Session Manager

Responsibilities:

- create and resume sessions
- track the active execution profile
- estimate cache warmth and prefix reuse
- calculate model-switch penalties
- preserve session policy version

Session affinity is a preference, not an absolute lock. A switch is allowed when:

```text
expected_gain > switch_penalty + context_replay_cost + risk_margin
```

### 7.3 Context Planner

Responsibilities:

- count and estimate tokens
- remove duplicate content
- identify reusable prefixes
- select relevant files or chunks
- estimate context replay cost
- determine whether the selected profile can accept the context

The first version may use deterministic trimming. Retrieval and reranking are later extensions.

### 7.4 Task Analyzer

The analyzer produces the normalized task profile.

It operates in stages:

```text
L0 hard constraints
  context length, tool requirement, modality, schema requirement

L1 local rules
  command flags, keywords, file types, known task templates

L2 local semantic classifier
  embedding similarity or lightweight classifier

L3 optional cheap LLM classifier
  only when ambiguity and expected savings justify its cost
```

The analyzer must record its own latency and cost.

### 7.5 Model Registry

The registry stores:

- provider metadata
- model metadata
- execution profiles
- prices
- context limits
- capabilities
- health and rate-limit history
- public benchmark seeds
- local runtime profiles

Free models use an explicit lifecycle:

```text
DISCOVERED
  -> SMOKE_TESTED
  -> CANDIDATE
  -> STABLE
  -> DEGRADED
  -> QUARANTINED
  -> DISABLED
```

### 7.6 Candidate Builder

The builder produces viable execution profiles.

Hard filters include:

- enabled provider
- healthy endpoint
- required context capacity
- required tools and modalities
- safety and domain restrictions
- task and daily budget
- free-model eligibility

It also creates two different recovery lists:

```text
quality escalation chain
reliability fallback chain
```

### 7.7 Cost Estimator and Ledger

The estimator predicts candidate cost before execution. The ledger records actual cost after execution.

Cost categories:

```text
routing_cost
input_cost
output_cost
thinking_cost
cache_write_cost
context_replay_cost
tool_cost
validation_cost
repair_cost
judge_cost
retry_cost
quality_escalation_cost
reliability_fallback_cost
rejected_work_cost
```

Primary metrics:

```text
trajectory_total_cost
cost_per_successful_trajectory
cost_per_accepted_trajectory
fallback_tax
context_replay_tax
routing_overhead_ratio
wasted_cost_ratio
```

### 7.8 Policy Engine

The policy engine evaluates candidates using constraints first and ranking second.

Optimization objective:

```text
minimize expected_trajectory_cost
subject to:
  expected_success_probability >= user_tolerance_floor
  safety_constraints == satisfied
  expected_latency <= configured_limit
  expected_cost <= budget
```

A useful candidate estimate is:

```text
expected_trajectory_cost
  = initial_call_cost
  + expected_validation_cost
  + probability_of_retry * retry_cost
  + probability_of_quality_escalation * escalation_cost
  + probability_of_reliability_failure * fallback_cost
  + switch_penalty
  + context_replay_cost
```

The router selects the lowest expected cost candidate satisfying all constraints.

### 7.9 Execution Coordinator

The coordinator executes the selected profile and emits structured events.

It is responsible for:

- timeouts
- streaming
- token accounting
- tool-call capture
- reasoning-token capture where available
- provider error normalization
- session affinity updates

### 7.10 Validation Coordinator

Validation order:

```text
1. deterministic parser or schema check
2. local formatter, compiler, lint, or test hook
3. task-specific acceptance criteria
4. cheap model repair when safe
5. LLM judge only when deterministic evidence is insufficient
```

`no_answer` is a valid structured outcome:

```json
{
  "status": "no_answer",
  "reason": "insufficient_context",
  "needs": ["more_files"]
}
```

It may trigger context expansion rather than immediate model escalation.

### 7.11 Escalation Manager

Three recovery categories are kept separate.

#### Quality escalation

The provider worked, but the result failed quality requirements.

#### Reliability fallback

The call failed because of timeout, rate limit, provider error, unavailable model, or network failure.

#### Budget fallback

The planned route would exceed the remaining budget and must be replaced or stopped.

These events update different profile statistics.

### 7.12 Feedback Service

Explicit feedback:

```text
accept
reject
rate
replay
manual_override
```

Implicit feedback:

```text
similar re-ask
immediate replay
large manual edit
forced model override
abandoned trajectory
```

Suggested evidence weights:

```text
explicit accept          +1.0
explicit reject          -1.0
manual replay            -0.7
explicit model override  -0.6
similar re-ask           -0.4
large manual edit        -0.3
```

Implicit signals remain weaker because intent is uncertain.

### 7.13 Evaluation Service

Evaluation has two layers.

#### Golden Set

Small, reviewed, stable cases containing:

```text
prompt
reference or baseline result
task family
difficulty
acceptance criteria
validators
```

#### Shadow Replay

Historical trajectory records are replayed against alternative policies or execution profiles.

Every trace must preserve:

```text
state -> candidates -> policy -> action -> outcome -> evaluation
```

This makes policy comparison possible without affecting the original user result.

### 7.14 Policy Lifecycle Manager

Policy states:

```text
DRAFT
VALIDATED
ACTIVE
DEGRADED
FROZEN
ROLLED_BACK
ARCHIVED
```

A learned adjustment creates a new draft. It does not directly mutate the active policy.

Activation requires:

- schema validation
- Golden Set regression pass
- routing overhead budget pass
- cost improvement or justified quality improvement
- no safety regression

## 8. End-to-End Execution Flow

```text
1. User submits a goal.
2. Session Manager loads session state and active policy version.
3. Trajectory Service creates a trajectory and opens the cost ledger.
4. Context Planner estimates context and replay cost.
5. Task Analyzer produces family, difficulty, domain, and constraints.
6. Candidate Builder filters execution profiles.
7. Cost Estimator predicts full trajectory cost for each candidate.
8. Policy Engine selects the cheapest acceptable candidate.
9. Execution Coordinator calls the provider.
10. Validation Coordinator validates the result.
11. Escalation Manager repairs, expands context, escalates, or falls back.
12. Trajectory reaches a terminal technical state.
13. CLI presents the result and trace summary.
14. Feedback Service records explicit or implicit feedback.
15. Profile Store updates evidence windows.
16. Policy learner may generate a new draft policy.
```

## 9. Coordination by Events

Business services coordinate through domain events rather than direct cross-module database writes.

Examples:

```text
TrajectoryStarted
TaskClassified
CandidatesBuilt
RoutingDecisionMade
ModelCallCompleted
ValidationFailed
QualityEscalated
ReliabilityFallbackTriggered
TrajectoryValidated
UserAccepted
UserRejected
PolicyDraftGenerated
PolicyActivated
PolicyRolledBack
```

Each event is persisted before derived profiles are updated. This supports auditability and replay.

## 10. Red Lines and Kill Switch

The local tool uses policy rollback rather than online traffic rollback.

Red lines:

```text
safety or privacy violation
Golden Set regression beyond configured threshold
re-ask rate anomaly
routing overhead above budget
cost per accepted trajectory regression
provider instability
```

Response:

```text
freeze learning
mark active policy DEGRADED
activate last stable policy
retain all traces for diagnosis
```

Rollback must not mean “always use the most expensive model.” It restores the last stable cost-first policy.

## 11. Local Storage Layout

```text
~/.grandet/
  config.yaml
  providers.yaml
  models.yaml
  user-profile.yaml
  grandet.db
  policies/
    stingy-v1.yaml
    stingy-v2.yaml
  evals/
    golden/
    regression/
    safety/
  traces/
  cache/
  logs/
```

## 12. First-Version Tradeoffs

### Chosen

- local CLI over hosted service
- SQLite over external database
- rule-based routing over learned routing
- task-level routing plus session affinity
- deterministic validation over universal LLM judging
- manual shadow replay over daemon-based continuous evaluation
- explicit policy versions over silent adaptive mutation

### Deferred

- step-level dynamic routing
- distributed execution
- online canary and A/B testing
- reinforcement learning
- automatic policy activation without user-visible evidence
- provider-specific cache internals that cannot be observed reliably

## 13. Success Criteria

GrandetAgent is successful when it can demonstrate all of the following:

```text
lower cost_per_accepted_trajectory than a fixed baseline
no material Golden Set regression
bounded routing overhead
bounded fallback tax
traceable and reversible policy changes
user-specific improvement over time
```

The project is not judged by how many providers it supports or how complex its Agent graph becomes. It is judged by how much unnecessary model spending it removes while preserving outcomes the user accepts.
