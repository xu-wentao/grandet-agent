# GrandetAgent Design Decisions

This document records key product and architecture decisions for GrandetAgent.

## Decision 1: GrandetAgent is not a general Agent orchestrator

GrandetAgent should not compete directly with LangGraph, LlamaIndex, Haystack, Dify, Semantic Kernel, or similar workflow frameworks.

GrandetAgent focuses on:

```text
local model routing + cost accounting + eval + user tolerance learning
```

Deferred:

- visual workflow builder
- complex multi-agent DAG editor
- enterprise orchestration platform
- generic low-code agent platform

## Decision 2: GrandetAgent is not a full AI Gateway replacement

GrandetAgent can use LiteLLM, OpenRouter, Portkey-like gateways, or any OpenAI-compatible endpoint, but it should not duplicate all gateway control-plane features in the first version.

GrandetAgent should add value above gateways:

- local task bucket classification
- local cost-per-accepted-task analysis
- local user feedback learning
- local shadow evaluation
- local routing recommendations

## Decision 3: Optimize cost per accepted task

Raw model call cost is not enough.

Primary metric:

```text
cost_per_accepted_task
```

This includes:

- initial model call
- retries
- repairs
- judges
- quality escalation
- reliability fallback
- rejected output waste

A cheap model with high failure or reject rate may have high effective cost.

## Decision 4: Split fallback into two types

GrandetAgent must distinguish quality and reliability.

### Quality escalation

The model was available, but the output was not good enough.

Examples:

- schema failure
- wrong answer
- user reject
- judge failure
- no_answer result

### Reliability fallback

The model or provider could not complete the request.

Examples:

- timeout
- rate limit
- provider error
- model unavailable
- context window error

The two signals should update model profiles differently.

## Decision 5: Use task buckets instead of only task types

Broad task types are too coarse.

Use `task_bucket` for routing and model profiles.

Initial buckets:

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

All runtime stats should be aggregated by model and task bucket.

## Decision 6: Start with rule-based routing

Do not implement a learned router in the first version.

Routing should start with transparent rules:

- task bucket
- context size
- required capabilities
- budget
- model status
- provider status
- historical accept rate
- fallback tax
- user tolerance floor

Later versions can add classifier-based routing or learned routing.

## Decision 7: Guardrail before escalation

Before escalating to a more expensive model, try deterministic validation and cheap repair.

Examples:

- JSON parser and JSON Schema
- YAML parser
- formatter / compiler / lint / tests
- tool argument schema validation
- static risk checks for shell commands

Model escalation should be used after cheap validation and repair options are exhausted or unsafe.

## Decision 8: Support no_answer as a valid output

Cheap models should be allowed to say they do not have enough context.

Protocol:

```json
{
  "status": "no_answer",
  "answer": null,
  "reason": "insufficient_context",
  "needs": ["more_files", "web_search", "stronger_reasoning_model"]
}
```

This prevents hallucination and gives the router a structured escalation signal.

## Decision 9: Public benchmarks are cold-start hints

Signal priority:

```text
user feedback > local runtime performance > local shadow eval > local eval suite > public benchmark > provider model card
```

Public benchmark data should initialize profiles, not dominate production routing.

## Decision 10: Shadow evaluation is core, but manual first

The first version should not require a daemon.

Shadow evaluation should be triggered by CLI:

```bash
grandet eval shadow --sample 20
grandet eval shadow --task-bucket summarization_short --models free,cheap
```

It should produce routing recommendations, not silently mutate behavior without trace.

## Decision 11: Free models require governance

Free models are not automatically trusted.

Lifecycle:

```text
FREE_DISCOVERED
  -> FREE_SMOKE_TESTED
  -> FREE_CANDIDATE
  -> FREE_STABLE
  -> FREE_DEGRADED
  -> FREE_DISABLED
```

Eligibility for real tasks should depend on health checks, smoke tests, rate-limit history, and task-bucket profile.

## Decision 12: Baseline savings must be explicit

GrandetAgent should support a configured baseline model to estimate savings.

Example:

```yaml
baseline:
  enabled: true
  model: openai/gpt-5.5
```

Savings analysis must clearly state whether baseline cost is estimated or measured.

## Decision 13: Context optimization is part of cost control

Reducing tokens is often cheaper than switching models.

First version context optimization:

- token estimation
- duplicate trimming
- history trimming
- file context size warnings

Later:

- chunking
- retrieval
- rerank
- compression
