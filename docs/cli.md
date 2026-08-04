# GrandetAgent CLI Design

## 1. Command Model

```bash
grandet <resource> <action> [options]
```

The CLI is a thin application boundary. It must not implement routing, learning, or persistence rules itself.

Global options:

```bash
--home <path>
--output text|json|yaml
--verbose
--trace
--no-color
```

## 2. Workspace

### init

```bash
grandet init
grandet init --dry-run
grandet init --force
grandet init --home ./tmp/.grandet
```

Creates:

- `config.yaml`, `providers.yaml`, `models.yaml`, and `user-profile.yaml` with an explicit `schema_version`
- `policies/stingy-v1.yaml` with its policy schema version
- `evals/golden/`, `evals/regression/`, and `evals/safety/`
- `grandet.db`, after transactional SQLite migrations

`--dry-run` lists every directory and file, including the database. Re-running initialization preserves generated files; `--force` refreshes only generated defaults and never removes unrelated files.

```text
~/.grandet/
  config.yaml
  providers.yaml
  models.yaml
  user-profile.yaml
  grandet.db
  policies/
  evals/
    golden/
    regression/
    safety/
  traces/
  cache/
  logs/
```

`init` applies SQLite migrations transactionally and records migration versions in `schema_versions`. A failed migration rolls back both its schema changes and version record; a later `init` reruns only pending migrations. Generated YAML files carry `schema_version: 1`; their recorded workspace versions are stored in SQLite. Re-running `init` preserves generated files, while `--force` refreshes only generated defaults and never removes unrelated files or resets `grandet.db`.

## 3. Configuration

```bash
grandet config show
grandet config edit
grandet config validate
grandet config set budget.daily_limit_usd 1.00
```

Configuration validation checks structure and references but does not make paid provider calls.

## 4. Providers

```bash
grandet provider list
grandet provider add openrouter \
  --base-url https://openrouter.ai/api/v1 \
  --key-env OPENROUTER_API_KEY
grandet provider test openrouter
grandet provider health openrouter
```

Provider test defaults to a non-generation health check where possible. A paid smoke call requires explicit confirmation or flag.

## 5. Models and Execution Profiles

### model

```bash
grandet model list
grandet model sync --provider openrouter
grandet model show <model-id>
grandet model enable <model-id>
grandet model disable <model-id>
grandet model smoke-test --free-only
grandet model promote <model-id>
grandet model quarantine <model-id>
grandet model clean-free
```

### profile

Routing selects execution profiles rather than raw model names.

```bash
grandet profile list
grandet profile show <profile-id>
grandet profile add --file profile.yaml
grandet profile enable <profile-id>
grandet profile disable <profile-id>
grandet profile compare <profile-a> <profile-b>
```

Example profile identifiers:

```text
openrouter-qwen-no-thinking
openrouter-qwen-low-reasoning
openai-mini-tool-enabled
```

## 6. Sessions

```bash
grandet session list
grandet session show <session-id>
grandet session new
grandet session close <session-id>
grandet session set-profile <session-id> <profile-id>
```

A session stores profile affinity, policy version, context fingerprint, and estimated continuity state.

## 7. Run a Trajectory

```bash
grandet run "Analyze this Kubernetes error" --profile <profile-id> --task-family debugging
grandet run --file task.md
grandet run --stdin
grandet run "Refactor this function" --context ./main.go
grandet run "Continue the previous analysis" --session <session-id>
grandet run "Use this exact profile" --profile <profile-id> --task-family summarization
grandet run "Hard budget example" --max-cost-usd 0.02
grandet run "Show full trace" --trace --verbose
```

Suggested concise output:

```text
Trajectory: trj_01J...
Task: debugging / difficulty 3 / kubernetes
Profile: openrouter-qwen-low-reasoning
Estimated trajectory cost: $0.0031
Actual trajectory cost: $0.0028
Session switch: no
Validation: passed
```

The current telemetry baseline requires `--profile`; `--task-family` records a specific family and defaults to the taxonomy's `general_qa`. It persists the session, trajectory, task, step, and `trajectory_started` event before provider work starts, then executes the selected enabled OpenAI-compatible profile. Provider fields absent from its response remain `unknown`; they are never fabricated. Run flags may appear before or after the prompt. The available baseline reports also accept `--session`, `--profile`, and `--outcome` filters alongside `--last`.

Verbose output includes:

- task analysis
- all candidate profiles
- filtered candidates and reasons
- expected trajectory cost
- switch and context-replay penalties
- quality escalation chain
- reliability fallback chain
- validators
- cost ledger

## 8. Trajectories and Tasks

The first implementation may retain `task` as the command namespace for compatibility, but the primary identifier is a trajectory ID.

```bash
grandet task list
grandet task show <trajectory-id>
grandet task trace <trajectory-id>
grandet task cost <trajectory-id>
grandet task replay <trajectory-id>
grandet task replay <trajectory-id> --profile <profile-id>
grandet task replay <trajectory-id> --policy <version>
```

Recommended later alias:

```bash
grandet trajectory show <trajectory-id>
```

The current debug classifier is local-only and does not require an initialized workspace or a model API:

```bash
grandet task classify "Summarize this report"
grandet task classify --tool kubectl --context pod.yaml --schema result.json "Diagnose this Kubernetes error"
```

It emits a versioned `TaskProfile`, including confidence and L0/L1 evidence for every field. `--task-family` is an explicit stable-taxonomy override.

## 9. Feedback

```bash
grandet accept <trajectory-id>
grandet reject <trajectory-id> --reason low_quality
grandet reject <trajectory-id> --reason wrong_answer
grandet reject <trajectory-id> --reason missing_detail
grandet reject <trajectory-id> --reason too_slow
grandet reject <trajectory-id> --reason too_expensive
grandet reject <trajectory-id> --reason format_wrong
grandet reject <trajectory-id> --reason tool_error
grandet rate <trajectory-id> --score 4
```

Feedback is append-only. It updates evidence and may produce a new policy draft, but it never silently changes the active policy.

## 10. Policy Management

```bash
grandet policy list
grandet policy show <version>
grandet policy validate <file-or-version>
grandet policy diff <version-a> <version-b>
grandet policy activate <version>
grandet policy health
grandet policy freeze
grandet policy rollback
grandet policy history
```

Policy output should show:

- status
- parent version
- supporting evidence
- Golden Set result
- expected cost change
- expected quality change
- rollback target

## 11. Evaluation

### Golden Set

```bash
grandet eval run --suite golden
grandet eval run --suite kubernetes
grandet eval compare --profiles <a>,<b>
grandet eval curve --strong <profile> --weak <profile>
grandet eval report <run-id>
```

### Shadow replay

```bash
grandet eval shadow --sample 20
grandet eval shadow --task-family debugging
grandet eval shadow --profiles free,cheap
grandet eval replay <trajectory-id> --policy <version>
grandet eval replay <trajectory-id> --profile <profile-id>
```

Shadow commands never overwrite original trajectories and must obey evaluation budgets and privacy settings.

## 12. Analysis

```bash
grandet analyze cost --today
grandet analyze cost --last 7d
grandet analyze savings --baseline <profile-id>
grandet analyze task-distribution --last 30d
grandet analyze profiles
grandet analyze rejects
grandet analyze reasks
grandet analyze routing
grandet analyze switching --session <session-id>
grandet analyze policy --version <version>
```

Cost analysis distinguishes:

```text
raw call cost
trajectory total cost
cost per successful trajectory
cost per accepted trajectory
fallback tax
context replay tax
routing overhead
rejected work
```

## 13. Context Utilities

```bash
grandet context estimate --file big.md
grandet context estimate --context ./repo
grandet context pack --context ./repo --query "fix node taint bug"
```

Initial versions may only estimate, deduplicate, and trim. Retrieval and reranking are later additions.

## 14. Database and Diagnostics

```bash
grandet doctor
grandet db status
grandet db migrate
grandet db verify
grandet version
```

`grandet doctor` should report:

- config paths
- schema version
- active policy
- enabled providers
- missing environment variables
- unhealthy models
- daily budget state

## 15. Output Stability

Text output is designed for humans and may evolve.

JSON output should use versioned schemas:

```json
{
  "schema_version": "v1",
  "data": {}
}
```

Automation should use JSON rather than parsing text tables.

## 16. Exit Codes

Suggested stable categories:

```text
0 success
2 invalid command or input
3 configuration error
4 provider reliability failure
5 validation or quality failure
6 budget exhausted
7 policy error
8 storage error
9 partial success with fallback
```

A fallback that still produces an accepted valid result should normally return success, while its trace records the fallback.
