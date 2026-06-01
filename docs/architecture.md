# GrandetAgent Architecture

GrandetAgent is a local-first, cost-aware Agent CLI.

## Positioning

GrandetAgent runs locally as a CLI tool. It stores configuration, model profiles, traces, task records, and user feedback under `~/.grandet/`.

The first version does not require a web dashboard, server gateway, multi-tenant system, Kubernetes deployment, or long-running daemon.

## Primary Goal

```text
Minimize cost while keeping the task likely to succeed under the current user's tolerance.
```

GrandetAgent has one default strategy:

```text
stingy: price first, fallback only when needed
```

## Core Principles

1. Price is the first priority.
2. Free models are preferred but must be profiled and monitored.
3. Cheap models are tried before expensive models.
4. User feedback overrides public benchmarks over time.
5. Every routing decision must be traceable.
6. Fallback cost is part of the real cost of a cheap model.

## Local Directory

```text
~/.grandet/
  config.yaml
  models.yaml
  providers.yaml
  user-profile.yaml
  grandet.db
  logs/
  traces/
  evals/
  cache/
```

## High-level Flow

```text
User command
  -> CLI command handler
  -> Config loader
  -> Local storage
  -> Task analyzer
  -> Stingy strategy engine
  -> Model router
  -> RunnerAgent
  -> Verifier / Judge
  -> Fallback if needed
  -> Final output
  -> Trace and cost record
  -> User feedback update
```

## Main Components

### Config Loader

Loads and merges:

```text
CLI flags > environment variables > user config files > defaults
```

Configuration files:

- `config.yaml`
- `providers.yaml`
- `models.yaml`
- `user-profile.yaml`

### Local Storage

The first version uses SQLite.

It stores:

- models
- model task profiles
- tasks
- model call spans
- routing decisions
- user feedback events
- shadow evaluation runs

### Provider Layer

The provider layer normalizes different LLM APIs.

Initial support:

- OpenAI-compatible APIs
- OpenRouter
- local Ollama later

Provider interface draft:

```go
type Provider interface {
    ListModels(ctx context.Context) ([]Model, error)
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    EstimateTokens(req ChatRequest) (TokenEstimate, error)
    HealthCheck(ctx context.Context) error
}
```

### Model Registry

The model registry maintains both static model information and runtime profiles.

Model states:

```text
DISCOVERED -> PROFILING -> ACTIVE -> DEGRADED -> DISABLED -> REMOVED
```

Free model states:

```text
FREE_UNTRUSTED
FREE_CANDIDATE
FREE_STABLE
FREE_DEGRADED
FREE_DISABLED
```

### Task Analyzer

The task analyzer estimates:

- task type
- difficulty
- expected input tokens
- expected output tokens
- required model capabilities
- reject risk
- current user tolerance floor

It must be cheap itself:

```text
rules -> local model -> free model -> cheapest paid model
```

### Stingy Strategy Engine

Objective:

```text
minimize(cost)
subject to:
  success_probability >= user_task_tolerance_floor
  model_available == true
  expected_cost <= configured_budget
```

Routing score draft:

```text
score =
  cost_score * 0.60
+ success_score * 0.20
+ user_acceptance_score * 0.12
+ latency_score * 0.05
+ stability_score * 0.03
- reject_penalty
- fallback_penalty
- rate_limit_penalty
```

Cost weight must remain the highest weight.

### Model Router

Filtering order:

1. model enabled
2. provider available
3. not rate-limited
4. required capability supported
5. context window sufficient
6. expected cost under budget
7. historical success rate satisfies user tolerance
8. sort by expected cost
9. cost tie-breaker: accept rate
10. final tie-breaker: latency

Routing output should include:

- selected model
- fallback chain
- estimated cost
- estimated latency
- selection reason
- reject risk
- filtered model reasons

### Verifier / Judge

Use deterministic validation before LLM judging:

| Task | Validation |
|---|---|
| JSON | JSON Schema |
| Code | compiler / lint / tests |
| Markdown | markdown lint |
| YAML | parser |
| Shell | static risk checks |

LLM judge is used only when deterministic checks are not enough.

### Feedback Updater

Supported feedback:

```text
accept
reject
rate
replay
manual_edit
```

Reject reasons:

```text
low_quality
wrong_answer
missing_detail
too_slow
too_expensive
format_wrong
tool_error
other
```

Feedback effects:

- accepted cheap model -> increase its task-specific weight
- rejected model -> decrease its task-specific weight
- repeated rejection -> raise quality floor for that task type
- too expensive -> increase cost penalty
- too slow -> increase latency penalty, but never above cost

## User Tolerance Learning

Example profile:

```yaml
user:
  id: local
  profile_version: v1

preferences:
  cost_sensitivity: 0.92
  quality_sensitivity: 0.58
  latency_sensitivity: 0.30
  default_acceptance_threshold: 0.62

task_tolerance:
  coding:
    min_success_probability: 0.78
    reject_rate_7d: 0.28
    accept_rate_7d: 0.62
    preferred_price_quantile: 0.20
  summarization:
    min_success_probability: 0.60
    reject_rate_7d: 0.07
    accept_rate_7d: 0.90
    preferred_price_quantile: 0.05
```

Learning must be conservative:

- single feedback event only makes a small adjustment
- repeated feedback has stronger effect
- recent 7-day data weighs more than older 30-day data
- minimum samples are required before strong routing changes

## Shadow Evaluation

The first version does not require a daemon.

Manual command:

```bash
grandet eval shadow --sample 20
```

Default privacy:

```yaml
privacy:
  allow_shadow_eval_for_user_tasks: false
  redact_before_eval: true
```

Shadow evaluation must have a strict budget.

## MVP Scope

Must have:

- `grandet init`
- local config directory
- SQLite storage
- OpenAI-compatible provider
- OpenRouter provider
- model list/sync/profile commands
- `grandet run`
- stingy router
- fallback chain
- feedback commands
- trace and cost analysis
- basic user tolerance update
- free model cleanup

Deferred:

- web dashboard
- server gateway
- Kubernetes deployment
- multi-tenancy
- daemon
- complex DAG agent
- full auto-learning algorithm
