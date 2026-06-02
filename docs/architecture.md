# GrandetAgent Architecture

GrandetAgent is a local-first, cost-aware Agent CLI.

Its purpose is not to become a large workflow orchestration platform or a generic AI gateway. It focuses on one narrow problem:

```text
Learn which cheapest model is good enough for each local task bucket.
```

## Positioning

GrandetAgent runs locally as a CLI tool. It stores configuration, model profiles, traces, task records, evaluation results, and user feedback under `~/.grandet/`.

The first version does not require:

- web dashboard
- server gateway
- multi-tenant control plane
- Kubernetes deployment
- long-running daemon
- complex visual workflow builder

GrandetAgent can integrate with existing gateways and providers, but it should not duplicate all of them.

Recommended relationship:

```text
Agent / User CLI
  -> GrandetAgent local router, cost accounting, eval, feedback learning
    -> OpenRouter / LiteLLM / OpenAI-compatible provider / local model runtime
```

## Primary Goal

The primary metric is not the cheapest single model call.

The primary metric is:

```text
cost_per_accepted_task
```

Definition:

```text
cost_per_accepted_task = total_task_cost / accepted_task_count
```

GrandetAgent optimizes for the average cost of tasks that the user actually accepts. This avoids fake savings where a cheap model has low per-call cost but causes many retries, fallbacks, and rejected outputs.

## Default Strategy

GrandetAgent has one default strategy:

```text
stingy: price first, but optimize for accepted task cost, not raw token price
```

The strategy is dynamic and local. It changes based on:

- task bucket
- model runtime profile
- deterministic validation result
- fallback history
- user accept / reject / rate feedback
- local shadow evaluation
- budget constraints

## Core Principles

1. Price is the first priority.
2. The effective cost is measured per accepted task, not per isolated call.
3. Free models are preferred but must be profiled, tested, and monitored.
4. Cheap models are tried before expensive models when they satisfy the current task bucket tolerance.
5. Quality escalation and reliability fallback are separate concepts.
6. Deterministic guardrails should run before expensive model escalation.
7. User feedback overrides public benchmarks over time.
8. Every routing decision must be traceable and explainable.
9. Public benchmarks are cold-start hints, not production truth.
10. Shadow evaluation is used to find cheaper substitutes without disrupting the main result.

## Reference Market Layering

GrandetAgent is designed around the current market split:

```text
Orchestration layer
  LangGraph / LlamaIndex / Haystack / Dify / Semantic Kernel

Gateway and routing layer
  LiteLLM / Portkey / OpenRouter / Vercel AI Gateway / RouteLLM

Eval and observability layer
  LangSmith / custom eval suites / production traces
```

GrandetAgent intentionally targets a local-first intersection:

```text
local task router + local cost accounting + local eval + user tolerance learning
```

It may call LiteLLM, OpenRouter, or any OpenAI-compatible endpoint, but its own differentiated value is the local decision loop.

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
  -> Context optimizer
  -> Task analyzer
  -> Task bucket classifier
  -> Stingy strategy engine
  -> Model router
  -> RunnerAgent
  -> Guardrail verifier
  -> Quality escalation if needed
  -> Reliability fallback if needed
  -> Final output
  -> Trace and cost record
  -> User feedback update
  -> Model and task-bucket profile update
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

Recommended baseline config:

```yaml
baseline:
  enabled: true
  model: openai/gpt-5.5
```

The baseline model is not used by default. It is used for savings analysis.

### Local Storage

The first version uses SQLite.

It stores:

- models
- model task profiles
- task bucket profiles
- tasks
- model call spans
- routing decisions
- fallback events
- user feedback events
- shadow evaluation runs
- baseline cost estimates

### Provider Layer

The provider layer normalizes different LLM APIs.

Initial support:

- OpenAI-compatible APIs
- OpenRouter
- LiteLLM proxy as an OpenAI-compatible provider
- local Ollama later
- local vLLM later

Example provider config:

```yaml
providers:
  litellm:
    type: openai_compatible
    base_url: http://localhost:4000/v1
    api_key_env: LITELLM_API_KEY
    enabled: false
```

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
FREE_DISCOVERED
  -> FREE_SMOKE_TESTED
  -> FREE_CANDIDATE
  -> FREE_STABLE
  -> FREE_DEGRADED
  -> FREE_DISABLED
```

A free model should not enter important real tasks only because its listed price is zero. It must pass health checks and task-bucket-specific smoke tests.

### Task Analyzer

The task analyzer estimates:

- task type
- task bucket
- difficulty
- context size
- expected input tokens
- expected output tokens
- required model capabilities
- verification mode
- reject risk
- current user tolerance floor

It must be cheap itself:

```text
rules -> local model -> free model -> cheapest paid model
```

### Task Bucket Classifier

GrandetAgent should aggregate runtime quality and cost by `task_bucket`, not only by broad task type.

Initial task buckets:

```text
chat_simple
classification
extraction_json
summarization_short
summarization_long
document_qa
coding_explain
coding_simple_patch
coding_complex_design
k8s_troubleshooting
log_analysis
```

Task profile draft:

```json
{
  "task_type": "coding",
  "task_bucket": "coding_simple_patch",
  "difficulty": "medium",
  "context_size": "medium",
  "requires_tool_calling": false,
  "requires_json_schema": false,
  "requires_long_context": false,
  "knowledge_mode": "provided_context",
  "verification_mode": "compile_or_static_check",
  "risk_level": "medium"
}
```

### Context Optimizer

The context optimizer reduces token waste before routing.

First version scope:

- estimate context tokens
- remove duplicate context blocks
- trim irrelevant history
- support explicit `--context` files
- warn when context exceeds model budget

Later scope:

- file chunking
- embedding retrieval
- rerank
- context packing
- prompt compression

Candidate commands:

```bash
grandet context estimate --file big.md
grandet context pack --context ./repo --query "fix node taint bug"
```

### Stingy Strategy Engine

Objective:

```text
minimize(cost_per_accepted_task)
subject to:
  success_probability >= user_task_bucket_tolerance_floor
  model_available == true
  expected_cost <= configured_budget
```

Routing score draft:

```text
score =
  cost_score * 0.55
+ accepted_task_cost_score * 0.20
+ task_bucket_success_score * 0.12
+ user_acceptance_score * 0.08
+ latency_score * 0.03
+ stability_score * 0.02
- reject_penalty
- fallback_tax_penalty
- rate_limit_penalty
```

Cost-related factors must remain dominant.

### Model Router

Filtering order:

1. model enabled
2. provider available
3. not rate-limited
4. required capability supported
5. context window sufficient
6. expected cost under budget
7. historical success rate satisfies the current task bucket tolerance
8. sort by expected effective cost
9. tie-breaker: accepted task cost
10. tie-breaker: accept rate
11. final tie-breaker: latency

Routing output should include:

- selected model
- quality escalation chain
- reliability fallback chain
- estimated raw call cost
- estimated task total cost
- estimated accepted task cost
- estimated fallback tax
- estimated latency
- selection reason
- reject risk
- filtered model reasons

### Quality Escalation vs Reliability Fallback

Do not collapse all fallback into one concept.

#### Quality Escalation

Quality escalation happens when the model result is not good enough.

Triggers:

- deterministic validation fails
- judge fails
- output violates schema
- answer returns `no_answer`
- task bucket has high reject rate for the selected model
- cheap model confidence is too low

Example:

```text
free model -> cheap paid model -> stronger model
```

#### Reliability Fallback

Reliability fallback happens when the model or provider cannot complete the call.

Triggers:

- timeout
- rate limit
- provider error
- context window error
- model unavailable
- network failure

Example:

```text
same model via another provider -> similar model via another provider -> next cheapest viable model
```

Each fallback event should record:

```text
fallback_type = quality_escalation | reliability_fallback | budget_fallback
```

### Guardrail Verifier

Use deterministic validation before LLM judging or model escalation.

| Task | Validation |
|---|---|
| JSON | JSON parser + JSON Schema |
| YAML | parser |
| Code | formatter / compiler / lint / tests |
| Markdown | markdown lint |
| Shell | static risk checks |
| Tool call | argument schema validation |

Guardrail-first behavior:

```text
format failure -> cheap repair or deterministic repair
missing context -> no_answer protocol
tool argument error -> schema-guided retry
code format error -> formatter before model escalation
```

LLM judge is used only when deterministic checks are not enough.

### no_answer Protocol

Cheap models should be allowed to refuse when context is insufficient instead of hallucinating.

Protocol draft:

```json
{
  "status": "no_answer",
  "answer": null,
  "reason": "insufficient_context",
  "needs": ["more_files", "web_search", "stronger_reasoning_model"]
}
```

Router behavior after `no_answer`:

- add context if available
- switch to long-context model
- trigger RAG or search when supported
- escalate to stronger model
- ask user for missing input when no safe route exists

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

- accepted cheap model -> increase its task-bucket weight
- rejected model -> decrease its task-bucket weight
- repeated rejection -> raise quality floor for that task bucket
- too expensive -> increase cost penalty
- too slow -> increase latency penalty, but never above cost
- manual edit -> treat as weak negative signal unless explicitly accepted

## Cost Accounting

GrandetAgent should track more than raw token cost.

Core cost fields:

```text
raw_call_cost
  Cost of one model call.

task_total_cost
  Sum of all calls, retries, judges, repairs, fallbacks, and tool costs for one task.

accepted_task_cost
  Cost of a task that the user accepted.

wasted_cost
  Cost spent on rejected, failed, invalid, or unused outputs.

fallback_tax
  Extra cost introduced by retries, repairs, judges, and fallbacks.

cost_per_accepted_task
  Total cost divided by accepted task count.
```

Model task profile should include:

```yaml
model_task_profile:
  task_bucket: coding_simple_patch
  raw_avg_cost_usd: 0.0004
  effective_cost_per_accepted_task_usd: 0.0021
  fallback_tax_rate: 0.18
  accept_rate: 0.74
  reject_rate: 0.16
  reliability_failure_rate: 0.04
  quality_escalation_rate: 0.10
```

## Baseline Savings Analysis

GrandetAgent should estimate savings against a configured baseline model.

Command:

```bash
grandet analyze savings --baseline openai/gpt-5.5
```

Example output:

```text
Total actual cost: $0.42
Estimated baseline cost: $5.83
Estimated savings: $5.41
Savings rate: 92.8%
Accepted task quality delta: -3.2%
```

Baseline analysis must be explicit and approximate unless the baseline actually ran.

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
  coding_simple_patch:
    min_success_probability: 0.78
    reject_rate_7d: 0.28
    accept_rate_7d: 0.62
    preferred_price_quantile: 0.20
  summarization_short:
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
- public benchmarks must not overpower local acceptance data

Profile signal priority:

```text
user feedback > local runtime performance > local shadow eval > local eval suite > public benchmark > provider model card
```

## Shadow Evaluation

The first version does not require a daemon. Shadow evaluation is manually triggered.

Command:

```bash
grandet eval shadow --sample 20
grandet eval shadow --task-bucket coding_simple_patch --sample 10
grandet eval shadow --task-bucket extraction_json --models free,cheap
```

Default privacy:

```yaml
privacy:
  allow_shadow_eval_for_user_tasks: false
  redact_before_eval: true
```

Shadow evaluation must have a strict budget.

Evaluation output should include:

- winner model
- accepted-quality cost
- latency
- fallback risk
- estimated savings
- recommended routing change

Example:

```text
For task_bucket=summarization_short:
  current preferred: deepseek-chat
  candidate: qwen-free
  quality delta: -2.1%
  cost delta: -100%
  latency delta: +0.8s
  recommendation: route more summarization_short tasks to qwen-free after 10 more samples
```

## Free Model Governance

Free models are useful but unstable. They need a lifecycle.

Commands:

```bash
grandet model smoke-test --free-only
grandet model clean-free
grandet model quarantine <model-id>
grandet model promote <model-id>
```

A free model can enter real tasks only when:

- smoke test passed
- recent health checks passed
- rate limit errors are under threshold
- task-bucket-specific profile is acceptable
- the task is not above the configured risk threshold

## MVP Scope

Must have:

- `grandet init`
- local config directory
- SQLite storage
- OpenAI-compatible provider
- OpenRouter provider
- LiteLLM-compatible provider example
- model list/sync/profile commands
- `grandet run`
- task bucket classification
- cost accounting
- baseline savings analysis
- stingy router
- separate quality escalation and reliability fallback
- feedback commands
- trace and cost analysis
- basic user tolerance update
- free model smoke test and cleanup

Deferred:

- web dashboard
- server gateway
- Kubernetes deployment
- multi-tenancy
- daemon
- complex DAG agent
- full auto-learning algorithm
- learned router
