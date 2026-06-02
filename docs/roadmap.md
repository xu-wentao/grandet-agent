# GrandetAgent Roadmap

## Milestone 0: Project Initialization

- README
- License
- Go module
- minimal CLI skeleton
- default config templates
- `grandet init`
- local `~/.grandet` directory creation
- CI workflow

## Milestone 1: Model and Provider Foundation

- provider config loader
- OpenAI-compatible client
- OpenRouter provider
- LiteLLM proxy as an OpenAI-compatible provider example
- model list command
- model sync command
- model enable / disable commands
- basic model profile format
- provider health check

## Milestone 2: Cost Accounting

Move cost accounting before advanced routing. GrandetAgent must know what it is optimizing before it routes.

- token estimation
- raw model call cost
- task total cost
- accepted task cost
- wasted cost
- fallback tax
- cost per accepted task
- baseline model configuration
- savings analysis against baseline

Commands:

```bash
grandet analyze cost --last 7d
grandet analyze savings --baseline openai/gpt-5.5
grandet task cost <task-id>
```

## Milestone 3: Rule-based Stingy Router

Start with transparent rules before learned routing.

- task bucket classification
- model capability filtering
- context window filtering
- budget filtering
- cheapest viable model selection
- quality escalation chain
- reliability fallback chain
- routing decision trace
- filtered model reasons

Initial task buckets:

- `chat_simple`
- `classification`
- `extraction_json`
- `summarization_short`
- `summarization_long`
- `document_qa`
- `coding_explain`
- `coding_simple_patch`
- `coding_complex_design`
- `k8s_troubleshooting`
- `log_analysis`

## Milestone 4: Task Execution, Trace, and Guardrails

- `grandet run`
- task records
- model call spans
- fallback events
- trace export
- concise and verbose output modes
- deterministic validators before LLM judging
- JSON / YAML validation
- basic code formatting or static check hooks
- `no_answer` protocol support

Commands:

```bash
grandet run "Analyze this Kubernetes error" --trace
grandet task trace <task-id>
```

## Milestone 5: Feedback Learning

- `grandet accept`
- `grandet reject`
- `grandet rate`
- feedback reasons
- user profile update
- task-bucket tolerance update
- model task profile update
- reject-rate-aware routing
- accepted-task-cost-aware routing

Commands:

```bash
grandet accept <task-id>
grandet reject <task-id> --reason wrong_answer
grandet rate <task-id> --score 4
```

## Milestone 6: Local Eval and Shadow Evaluation

- local eval suite format
- baseline eval
- shadow eval over historical tasks
- compare free / cheap / baseline models
- estimated routing recommendation
- strict shadow eval budget
- privacy controls and redaction hooks

Commands:

```bash
grandet eval run --suite coding-basic
grandet eval shadow --sample 20
grandet eval shadow --task-bucket summarization_short --models free,cheap
grandet eval report
```

## Milestone 7: Free Model Governance

- free model lifecycle state machine
- smoke test command
- quarantine command
- promote command
- clean-free command
- rate-limit based degradation
- reliability failure tracking
- task-bucket-specific free model eligibility

Commands:

```bash
grandet model smoke-test --free-only
grandet model clean-free
grandet model quarantine <model-id>
grandet model promote <model-id>
```

## Milestone 8: Context Optimizer

- token estimation for files and prompts
- duplicate context trimming
- context size warnings
- simple context packing
- later: chunking, embedding retrieval, rerank, compression

Commands:

```bash
grandet context estimate --file big.md
grandet context pack --context ./repo --query "fix node taint bug"
```

## Deferred

- web dashboard
- OpenAI-compatible server mode
- daemon mode
- multi-tenant support
- Kubernetes deployment
- Prometheus / Grafana integration
- complex DAG-based multi-agent orchestration
- learned router
- distillation pipeline
