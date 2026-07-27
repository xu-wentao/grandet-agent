# Contributing to GrandetAgent

GrandetAgent is an early-stage Go 1.22, local-first CLI. Keep changes narrow, verify them locally, and keep implemented behavior distinct from planned behavior in `docs/`.

## Local setup and checks

```bash
go mod tidy
test -z "$(gofmt -l ./cmd ./internal)"
go test ./...
go build ./cmd/grandet
go run ./cmd/grandet --help
go run ./cmd/grandet init --dry-run
```

For an initialization that writes files, use a temporary directory rather than your normal home:

```bash
go run ./cmd/grandet init --home "$(mktemp -d)/.grandet"
```

`make build`, `make test`, `make fmt`, and `make tidy` are convenience wrappers for the corresponding Go commands.

## Package boundaries

- `cmd/grandet`: binary entry point only.
- `internal/cli`: argument parsing and terminal output; do not put domain rules here.
- `internal/application`: use cases and transaction boundaries.
- `internal/domain`: business rules and types; it must not depend on CLI, SQL, or provider-specific packages.
- `internal/infrastructure`: filesystem, SQLite, clocks, identifiers, and other adapters.

The intended dependency direction is `CLI -> Application -> Domain <- Infrastructure`. Prefer the standard library and add a dependency only when the existing module cannot do the job.

## Issues, pull requests, and documentation

Use the issue templates for features, bugs, and design decisions. Name a branch after the issue, for example `feat/42-provider-health`, and open a PR with `Closes #42` when it fully resolves that issue.

Prefer small vertical PRs: one observable behavior, its tests, and the documentation/configuration it changes. Do not mix a refactor with an unrelated feature.

For a behavior, schema, policy, provider, cost, or trace change, update the relevant document in `docs/` and any matching `configs/*.example.yaml`. Planned commands in the design documents are not evidence that a command is implemented.

Migrations must be backward-compatible: preserve existing workspace data, make the change idempotent where possible, and leave a clear upgrade path. Do not remove or rewrite existing user configuration without an explicit, documented migration and a compatibility check.

## Pull request checklist

- Link the issue and describe the user-visible change.
- State design, cost, trace, policy, schema, provider, migration, and configuration impact (or say none).
- List the commands run and any manual verification.
- Keep the title imperative and narrowly scoped, using the repository's Conventional Commit style when practical.

See the [release checklist](docs/release-checklist.md) before calling either MVP ready.
