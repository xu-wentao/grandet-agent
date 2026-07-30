# GrandetAgent

GrandetAgent is a local-first, cost-optimizing Agent CLI.

Its purpose is:

> Coordinate the cheapest execution profiles that are still likely to produce an acceptable result for the complete Agent trajectory.

GrandetAgent is not interested in the cheapest isolated model call. It tracks the full cost of routing, reasoning, context replay, tools, validation, repair, retry, escalation, fallback, and rejected work.

The primary metric is:

```text
cost_per_accepted_trajectory
```

## Core Philosophy

- Price is the first priority; success and safety are constraints.
- A trajectory, not a single call, is the economic unit.
- The router selects execution profiles, not only model names.
- Turning reasoning down may be cheaper than switching models.
- Session continuity and context replay have economic value.
- Routing itself must cost less than it saves.
- Deterministic validators run before expensive judges or model escalation.
- Public benchmarks initialize profiles; local evidence eventually overrides them.
- User feedback creates versioned policy drafts rather than silently changing behavior.
- Every routing and policy decision is inspectable and reversible.

## First-Version Shape

GrandetAgent v0.x is a local CLI, not a hosted platform.

```bash
grandet <resource> <action> [options]
```

Local data is stored under:

```text
~/.grandet/
  config.yaml
  providers.yaml
  models.yaml
  user-profile.yaml
  grandet.db
  policies/
  evals/
  traces/
  cache/
  logs/
```

## Planned Capabilities

- OpenAI-compatible and OpenRouter providers
- model discovery and free-model governance
- model execution profiles with reasoning modes
- session affinity and model-switch penalties
- task-family and difficulty classification
- full trajectory cost ledger
- rule-based stingy routing
- deterministic validation and cheap repair
- separate quality escalation and reliability fallback
- explicit and implicit user feedback
- versioned policy validation, activation, and rollback
- Golden Set evaluation and historical shadow replay

## Example CLI

```bash
grandet init

grandet provider list
grandet model sync --provider openrouter
grandet profile list

grandet run "Analyze this Kubernetes error" --trace

grandet task cost <trajectory-id>
grandet accept <trajectory-id>
grandet reject <trajectory-id> --reason wrong_answer

grandet policy health
grandet eval run --suite golden
grandet eval shadow --sample 20

grandet analyze savings --baseline <profile-id>
```

## Documentation

- [Architecture and philosophy](docs/architecture.md)
- [Business-layer implementation logic](docs/business-logic.md)
- [Domain data model](docs/data-model.md)
- [Design decisions and tradeoffs](docs/design-decisions.md)
- [CLI design](docs/cli.md)
- [Roadmap](docs/roadmap.md)

## Current Status

The repository has the workspace foundation plus an OpenAI-compatible provider health slice. `grandet init` creates versioned YAML defaults and a migration-managed SQLite database. `grandet provider list` loads `providers.yaml`; `grandet provider test <name>` checks its `/models` endpoint without generating tokens. Credentials are read only from the configured environment variable and are never stored. Existing generated files are preserved unless `--force` is used; unrelated workspace files are never removed.

The implementation sequence intentionally starts with telemetry, baseline measurement, and trajectory accounting before advanced routing. GrandetAgent must first prove what it is optimizing.

## Development

```bash
go mod tidy
go run ./cmd/grandet --help
go run ./cmd/grandet init --dry-run
```

## License

Apache-2.0
