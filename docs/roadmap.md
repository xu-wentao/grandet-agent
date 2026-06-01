# GrandetAgent Roadmap

## Milestone 0: Project Initialization

- README
- License
- Go module
- cobra CLI skeleton
- default config templates
- `grandet init`
- local `~/.grandet` directory creation

## Milestone 1: Model and Provider Foundation

- provider config loader
- OpenAI-compatible client
- OpenRouter provider
- model list command
- model sync command
- model enable / disable commands
- basic model profile format

## Milestone 2: Stingy Router

- token estimation
- cost estimation
- model candidate filtering
- cheapest viable model selection
- fallback chain
- routing decision trace

## Milestone 3: Task Execution and Trace

- `grandet run`
- task records
- model call spans
- trace export
- task cost report
- concise and verbose output modes

## Milestone 4: Feedback Learning

- `grandet accept`
- `grandet reject`
- `grandet rate`
- user profile update
- model task profile update
- reject-rate-aware routing

## Milestone 5: Local Eval and Free Model Governance

- `grandet eval shadow`
- `grandet eval report`
- `grandet model clean-free`
- free model state machine
- shadow eval budget control

## Deferred

- web dashboard
- OpenAI-compatible server mode
- daemon mode
- multi-tenant support
- Kubernetes deployment
- Prometheus / Grafana integration
- complex DAG-based multi-agent orchestration
