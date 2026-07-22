# GrandetAgent Design Decisions

This document records the current architectural decisions and the reasons behind them.

## Decision 1: The economic unit is a trajectory

GrandetAgent does not optimize isolated model calls.

Primary metrics:

```text
cost_per_successful_trajectory
cost_per_accepted_trajectory
```

A trajectory includes routing, context preparation, model calls, tools, validation, repair, retry, escalation, fallback, and rejected work.

**Tradeoff:** accounting becomes more complex, but it prevents cheap calls with expensive failure paths from appearing efficient.

## Decision 2: Price is first, success is a constraint

The default policy is always stingy:

```text
minimize expected trajectory cost
subject to success, safety, latency, and budget constraints
```

GrandetAgent does not expose several equal first-class modes such as balanced, premium, or quality-first in the initial design.

**Tradeoff:** the product is less generic, but its behavior and value proposition stay clear.

## Decision 3: Route execution profiles, not model names

A routing candidate is:

```text
provider + model + reasoning mode + token limits + tool mode + retry policy
```

The same model can have cheap and expensive execution profiles.

**Reason:** turning reasoning off or reducing output limits may save more money and preserve continuity better than switching model families.

## Decision 4: Use task family and difficulty as separate axes

Profiles are not aggregated only by broad buckets such as `coding`.

Primary classification:

```text
task_family × difficulty
```

Additional dimensions include domain, risk, seriousness, context size, tools, and verification mode.

**Tradeoff:** more dimensions require more samples, so the system must back off to coarser profiles when evidence is sparse.

## Decision 5: Start with task-level routing and session affinity

The initial implementation routes at task boundaries and preserves model affinity inside a session.

SubAgent and step-level routing are deferred.

**Reason:** very fine-grained switching can destroy cache value, replay context, and create more routing overhead than savings.

## Decision 6: Cache and context replay are economic inputs

Even when exact provider KV-cache state is unavailable, GrandetAgent estimates:

```text
context_replay_tokens
model_switch_penalty
estimated_cache_eviction_cost
```

A cheaper candidate is rejected when switching cost exceeds expected savings.

**Tradeoff:** estimates may be imperfect, but ignoring these costs is systematically worse.

## Decision 7: Routing must have its own budget

Every analyzer and classifier records latency and cost.

Default goals:

```text
rules and hard constraints: near zero cost
local semantic classifier: bounded local latency
LLM classifier: disabled unless expected value is positive
```

The policy may enforce:

```text
routing_overhead_ratio <= configured threshold
```

**Reason:** a router that costs more than it saves violates the project philosophy.

## Decision 8: Use layered classification

Classification order:

```text
L0 hard constraints
L1 local rules
L2 local embedding or lightweight classifier
L3 optional cheap LLM classifier
```

Each later layer is entered only when earlier layers are insufficient and potential savings justify the added cost.

## Decision 9: Deterministic validation precedes model escalation

GrandetAgent uses parsers, schemas, formatters, compilers, linters, tests, and tool-argument validation before paying for another model.

**Tradeoff:** validators require task-specific integrations, but they are cheaper and more trustworthy than universal LLM judging.

## Decision 10: `no_answer` is a valid result

A model may return structured insufficient-context information instead of hallucinating.

```json
{
  "status": "no_answer",
  "reason": "insufficient_context",
  "needs": ["more_files"]
}
```

The system may expand context before escalating model quality.

## Decision 11: Quality escalation and reliability fallback are different

### Quality escalation

The provider completed the call, but the result did not satisfy quality requirements.

### Reliability fallback

The provider or model failed technically.

### Budget fallback

The route cannot continue within the remaining budget.

These events update separate statistics.

**Reason:** a timeout should not lower model quality reputation, and a wrong answer should not lower provider reliability reputation.

## Decision 12: User feedback is task-specific

User tolerance is learned by task family, difficulty, and domain rather than as one global quality preference.

A user may accept cheap summaries but reject cheap architecture advice.

**Tradeoff:** learning is slower, but resulting behavior is more faithful.

## Decision 13: Explicit feedback is stronger than implicit feedback

Evidence weights are ordered approximately as:

```text
explicit accept or reject
manual replay or model override
similar re-ask
large manual edit
abandonment
```

Implicit signals remain weak because their intent is ambiguous.

## Decision 14: Learning is conservative and versioned

Feedback updates evidence windows and may produce a new policy draft.

It never mutates the active policy invisibly.

Policy lifecycle:

```text
DRAFT -> VALIDATED -> ACTIVE -> DEGRADED -> FROZEN -> ROLLED_BACK
```

**Tradeoff:** adaptation is slower, but every behavioral change is inspectable and reversible.

## Decision 15: Public benchmarks are cold-start priors

Evidence priority:

```text
user feedback
  > validated local outcomes
  > implicit behavior
  > shadow replay
  > Golden Set
  > public benchmark
  > provider metadata
```

A high public score cannot override repeated local failures.

## Decision 16: Golden Set cases contain acceptance criteria

A Golden Set case is not only a prompt and reference answer.

It includes:

```text
task family
difficulty
acceptance criteria
validators
safety requirements
```

**Reason:** Agent tasks are often better judged by required behavior and tool correctness than by text similarity.

## Decision 17: Shadow replay is manual first

The initial CLI supports explicit replay and comparison commands.

It does not require a background daemon.

Every stored decision preserves:

```text
state -> candidates -> policy -> action -> outcome -> evaluation
```

This supports later policy comparison and learned routing.

## Decision 18: LLM judges are offline-biased

Main-path evaluation prefers deterministic validators and at most a cheap judge when necessary.

Golden Set and shadow evaluation may use stronger pairwise judges, answer-order swapping, cross-family judges, and human calibration.

**Reason:** using multiple expensive judges on every live task would destroy savings.

## Decision 19: Free models require admission and lifecycle management

Free models move through:

```text
DISCOVERED
SMOKE_TESTED
CANDIDATE
STABLE
DEGRADED
QUARANTINED
DISABLED
```

They are eligible for real tasks only when provider health and task-specific evidence are sufficient.

## Decision 20: The active policy has a kill switch

Triggers may include:

- safety violation
- Golden Set regression
- abnormal re-ask rate
- routing overhead regression
- cost-per-accepted-trajectory regression
- provider instability

The response is to freeze learning and restore the last stable stingy policy, not to route everything to the most expensive model.

## Decision 21: GrandetAgent is not a general Agent framework

It does not compete with workflow frameworks on graph authoring or low-code orchestration.

Its differentiated responsibility is:

```text
local trajectory economics + model execution routing + evaluation + user-specific learning
```

## Decision 22: GrandetAgent is not a full gateway replacement

It may use OpenRouter, LiteLLM, OpenAI-compatible APIs, and local runtimes.

It avoids duplicating enterprise gateway control-plane features in the first version.

## Decision 23: CLI and SQLite remain the first implementation form

The first version stays local and inspectable.

Deferred:

- hosted server mode
- multi-tenancy
- distributed execution
- online canary infrastructure
- visual dashboard
- Kubernetes deployment
- reinforcement learning policy control
