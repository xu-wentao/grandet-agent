# GrandetAgent

GrandetAgent is a local-first, cost-aware Agent CLI.

Its goal is simple:

> Use the cheapest possible model that is still likely to complete the task successfully.

GrandetAgent does not default to the strongest model. It routes each task through a stingy model-selection strategy that prefers free and low-cost models, uses fallback only when needed, and learns from local user feedback such as `accept`, `reject`, and `rate`.

## Core Ideas

- Price is the first priority.
- Free models are preferred, but not blindly trusted.
- Cheap models are tried before expensive models.
- Fallback is used when quality, validation, or user tolerance requires it.
- Every task must be traceable: selected model, fallback chain, estimated cost, actual cost, latency, and user feedback.
- User feedback gradually changes local task tolerance and model routing weights.

## First Version Scope

GrandetAgent v0.x is a local CLI, not a web platform.

```bash
grandet <command> [options]
```

The first version focuses on:

- local configuration under `~/.grandet/`
- OpenAI-compatible providers
- OpenRouter model discovery
- manual model configuration
- SQLite local storage
- stingy model routing
- fallback chains
- task trace and cost analysis
- user feedback learning
- free model cleanup

## Example Commands

```bash
grandet init

grandet provider list
grandet model sync --provider openrouter
grandet model list

grandet run "Summarize this document" --context ./docs/architecture.md --trace

grandet accept <task-id>
grandet reject <task-id> --reason low_quality
grandet rate <task-id> --score 4

grandet task trace <task-id>
grandet analyze cost --last 7d
```

## Repository Status

This repository is in early design and initialization.

Current focus:

1. CLI skeleton
2. local config initialization
3. model/provider config format
4. SQLite storage schema
5. cost-first routing design

## Documentation

- [Architecture](docs/architecture.md)
- [CLI Design](docs/cli.md)
- [Roadmap](docs/roadmap.md)

## Development

```bash
go mod tidy
go run ./cmd/grandet --help
go run ./cmd/grandet init --dry-run
```

## License

Apache-2.0
