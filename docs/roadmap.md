# GrandetAgent Roadmap

The roadmap follows one rule:

> Measure real trajectory economics before attempting increasingly intelligent routing.

Each milestone must leave the system usable and inspectable through the CLI.

## Milestone 0: Project and Domain Foundation

Status: initial skeleton exists.

Deliverables:

- Go module and minimal CLI
- `grandet init`
- local `~/.grandet/` layout
- configuration templates
- CI workflow
- domain package boundaries
- SQLite migration framework
- stable ID and clock abstractions

Exit criteria:

- initialization is idempotent
- migrations are tested
- CLI, application, domain, and infrastructure dependencies are separated

## Milestone 1: Telemetry and Baseline

Do not build an intelligent router before knowing the request distribution and current cost.

Deliverables:

- session records
- trajectory records
- task and step records
- append-only trajectory events
- token and latency collection
- tool-call outcome collection
- reasoning-token collection where available
- provider error normalization
- baseline execution profile configuration
- baseline cost and task-distribution reports

Commands:

```bash
grandet run --profile <baseline-profile> --task-family <family> "..."
grandet analyze cost --last 7d
grandet analyze task-distribution --last 30d
```

Exit criteria:

- every paid call belongs to a trajectory
- input, output, reasoning, retry, and fallback costs can be attributed
- a fixed-profile baseline can be measured

## Milestone 2: Provider, Model, and Execution Profile Registry

Deliverables:

- provider configuration loader
- OpenAI-compatible provider adapter
- OpenRouter adapter
- LiteLLM-compatible example
- model discovery and synchronization
- versioned price history
- capability normalization
- execution profiles with reasoning modes and limits
- provider health checks
- manual enable, disable, and quarantine operations

Commands:

```bash
grandet provider list
grandet provider test <provider>
grandet model sync --provider openrouter
grandet profile list
grandet profile show <profile-id>
```

Exit criteria:

- router-facing code sees execution profiles rather than raw provider models
- the same model may expose no-thinking and reasoning-enabled profiles
- historical price changes remain traceable

## Milestone 3: Trajectory Cost Ledger

Deliverables:

- cost ledger entries
- raw call cost
- task and trajectory total cost
- reasoning cost
- context replay cost estimate
- routing overhead
- validation and judge cost
- retry and repair cost
- quality-escalation and reliability-fallback cost
- rejected-work cost
- baseline savings estimates

Primary metrics:

```text
cost_per_successful_trajectory
cost_per_accepted_trajectory
fallback_tax
context_replay_tax
routing_overhead_ratio
```

Commands:

```bash
grandet task cost <trajectory-id>
grandet analyze cost --last 7d
grandet analyze savings --baseline <profile-id>
```

Exit criteria:

- reports no longer rely on per-call cost alone
- estimates and measured values are visibly distinguished

## Milestone 4: Task Taxonomy and Rule-Based Router

Start with transparent, low-overhead routing.

Deliverables:

- task-family classification
- 1-5 difficulty classification
- domain, risk, seriousness, and context-size features
- L0 hard constraints
- L1 local rule classifier
- candidate filtering
- expected trajectory cost estimation
- cheapest acceptable profile selection
- routing-decision and rejected-candidate traces

Initial task families:

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

Exit criteria:

- no LLM classifier is required for normal routing
- routing overhead is measured and bounded
- every candidate rejection has an explanation

## Milestone 5: Session Affinity and Cache-Aware Economics

Deliverables:

- session resume support
- active profile affinity
- context fingerprinting
- context replay token estimation
- model switch penalty
- reasoning-mode change before model-family switch
- session switch analysis report

Commands:

```bash
grandet session show <session-id>
grandet analyze switching --session <session-id>
```

Exit criteria:

- a superficially cheaper model is not selected when switch and replay costs destroy savings
- affinity can be overridden explicitly

## Milestone 6: Execution, Guardrails, and Escalation

Deliverables:

- `grandet run`
- streaming and timeout handling
- JSON and YAML validation
- tool argument validation
- formatter, compiler, lint, and test hooks
- `no_answer` protocol
- deterministic repair
- same-profile constrained retry
- context expansion
- separate quality-escalation chain
- separate reliability-fallback chain
- hard budget handling

Commands:

```bash
grandet run "..." --context <path> --trace
grandet task trace <trajectory-id>
```

Exit criteria:

- reliability failures do not damage quality statistics
- formatting failures do not automatically invoke premium models
- hard budgets are never silently exceeded

## Milestone 7: Feedback and User Tolerance

Deliverables:

- explicit accept, reject, and rate commands
- manual replay and model override signals
- implicit re-ask detection
- recent and stable evidence windows
- user tolerance by task family, difficulty, and domain
- execution-profile performance profiles
- expected escalation and accepted-trajectory-cost updates

Commands:

```bash
grandet accept <trajectory-id>
grandet reject <trajectory-id> --reason wrong_answer
grandet rate <trajectory-id> --score 4
grandet task replay <trajectory-id>
```

Exit criteria:

- one isolated feedback event cannot radically change routing
- explicit feedback is stronger than implicit feedback
- profile evidence is inspectable

## Milestone 8: Policy Versioning and Kill Switch

Deliverables:

- policy YAML schema
- policy repository
- draft generation from evidence
- static validation
- version diff
- explicit activation
- active-policy health report
- freeze and rollback
- last-stable-policy recovery

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

Commands:

```bash
grandet policy list
grandet policy validate <file>
grandet policy diff <v1> <v2>
grandet policy activate <version>
grandet policy health
grandet policy rollback
```

Exit criteria:

- learning never mutates the active policy invisibly
- every activation has validation evidence and a rollback target

## Milestone 9: Golden Set and Offline Evaluation

Deliverables:

- Golden Set YAML format
- initial 30 reviewed cases
- acceptance criteria and validator definitions
- general, coding, and Kubernetes/SRE suites
- baseline strong and weak profile runs
- quality retention
- PGR/APGR/CPT where applicable
- judge bias controls for offline pairwise evaluation
- regression and safety suites

Commands:

```bash
grandet eval run --suite golden
grandet eval compare --profiles <a>,<b>
grandet eval curve --strong <profile> --weak <profile>
```

Exit criteria:

- policy changes can be evaluated before activation
- judge and validator disagreements are visible

## Milestone 10: Shadow Replay

Deliverables:

- historical state snapshots
- replay by policy version
- replay by execution profile
- reasoning-mode comparisons
- preserved original trajectory
- cost, quality, latency, and fallback-tax deltas
- routing recommendations
- strict evaluation budget and privacy controls

Commands:

```bash
grandet eval shadow --sample 20
grandet eval shadow --task-family debugging
grandet eval replay <trajectory-id> --policy <version>
```

Exit criteria:

- a candidate policy can be compared without changing the original user outcome
- shadow results generate recommendations, not silent activation

## Milestone 11: Free Model Governance

Deliverables:

- lifecycle state machine
- smoke tests
- task-family admission rules
- rate-limit and timeout degradation
- price-change handling
- quarantine, promote, and cleanup commands

Commands:

```bash
grandet model smoke-test --free-only
grandet model promote <model-id>
grandet model quarantine <model-id>
grandet model clean-free
```

Exit criteria:

- newly discovered free models cannot directly enter high-risk tasks
- free-model value includes failure and fallback tax

## Milestone 12: Local Semantic Router

Only begin after enough labeled local data exists.

Deliverables:

- embedding-based task matching
- lightweight difficulty classifier
- classifier confidence calibration
- routing-value gate
- rule and classifier disagreement analysis
- offline comparison against the rule router

Exit criteria:

- classifier improves the cost-quality frontier
- classifier cost and latency stay within router budget

## Milestone 13: Context Optimization

Deliverables:

- duplicate context removal
- history trimming
- stable-prefix detection
- simple context packing
- later: chunking, retrieval, reranking, compression

Commands:

```bash
grandet context estimate --file <path>
grandet context pack --context <path> --query "..."
```

## Deferred Beyond the Local CLI

- OpenAI-compatible server mode
- background daemon and scheduled evaluation
- multi-user service
- centralized dashboard
- online canary and A/B infrastructure
- Kubernetes deployment
- distributed workers
- step-level learned routing
- reinforcement learning policy control
- automatic policy activation without explicit evidence
