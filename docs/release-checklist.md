# Release Checklist

Use this checklist for a release candidate. It separates the implemented baseline from the later router capability described in the roadmap.

## Baseline MVP

- [ ] `go mod tidy` leaves no diff.
- [ ] `test -z "$(gofmt -l ./cmd ./internal)"` passes.
- [ ] `go test ./...` and `go build ./cmd/grandet` pass.
- [ ] `go run ./cmd/grandet --help` and `go run ./cmd/grandet init --dry-run` match the documented CLI.
- [ ] A fresh temporary `--home` workspace initializes, and rerunning it preserves generated files unless `--force` is used.
- [ ] Default YAML files and SQLite migration versions are recorded and documented.
- [ ] Documentation distinguishes the implemented workspace foundation from planned commands.

## Router MVP

- [ ] Baseline MVP checks still pass.
- [ ] Every paid call belongs to a persisted trajectory with attributable cost.
- [ ] Routing selects an execution profile and records selected and rejected candidates.
- [ ] Routing overhead, context replay, retries, validation, escalation, and fallback costs are visible in the trace or ledger.
- [ ] Deterministic validation runs before quality escalation; reliability fallback remains separately classified.
- [ ] Policy and schema/config migrations are backward-compatible, documented, and have a rollback path.
- [ ] A focused Golden Set or equivalent regression check covers the changed routing behavior.

The detailed scope and sequencing live in the [architecture](architecture.md), [business logic](business-logic.md), and [roadmap](roadmap.md).
