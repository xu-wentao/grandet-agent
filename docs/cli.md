# GrandetAgent CLI Design

## Command Overview

```bash
grandet <command> [options]
```

## init

Initialize the local GrandetAgent workspace.

```bash
grandet init
grandet init --dry-run
grandet init --force
```

Creates:

```text
~/.grandet/
  config.yaml
  providers.yaml
  models.yaml
  user-profile.yaml
  grandet.db
  logs/
  traces/
  evals/
  cache/
```

## config

```bash
grandet config show
grandet config edit
grandet config set strategy.default_quality_floor low_medium
grandet config set budget.daily_limit_usd 1.00
```

## provider

```bash
grandet provider list
grandet provider add openrouter --base-url https://openrouter.ai/api/v1 --key-env OPENROUTER_API_KEY
grandet provider test openrouter
```

## model

```bash
grandet model list
grandet model sync --provider openrouter
grandet model add --id deepseek/deepseek-chat --provider deepseek
grandet model profile openrouter/qwen/qwen3-coder-free
grandet model disable <model-id>
grandet model enable <model-id>
grandet model clean-free
```

## run

```bash
grandet run "Analyze this Kubernetes error"
grandet run --file task.md
grandet run --stdin
grandet run "Refactor this function" --context ./main.go
grandet run "Summarize this architecture" --context ./docs/architecture.md --trace
```

Suggested default output:

```text
Selected model: openrouter/qwen/qwen3-coder-free
Estimated cost: $0.0000
Fallback: enabled
Reason: free model met minimum profile for summarization
```

Use `--verbose` to print the full routing process.

## feedback

```bash
grandet accept <task-id>
grandet reject <task-id>
grandet reject <task-id> --reason low_quality
grandet reject <task-id> --reason wrong_answer
grandet reject <task-id> --reason too_slow
grandet reject <task-id> --reason too_expensive
grandet rate <task-id> --score 4
```

Feedback reasons:

```text
low_quality
wrong_answer
missing_detail
too_slow
too_expensive
format_wrong
tool_error
other
```

## task

```bash
grandet task list
grandet task show <task-id>
grandet task trace <task-id>
grandet task cost <task-id>
grandet task replay <task-id> --model <model-id>
```

## analyze

```bash
grandet analyze cost --today
grandet analyze cost --last 7d
grandet analyze models
grandet analyze task-types
grandet analyze rejects
grandet analyze savings
grandet analyze routing
```

## eval

```bash
grandet eval run --suite coding-basic
grandet eval shadow --sample 20
grandet eval shadow --task-type coding --sample 10
grandet eval report
grandet eval compare --models model-a,model-b,model-c
```

## Output Modes

Initial output modes:

```bash
--output text
--output json
--output yaml
```

Default output should be concise text. Use JSON for scripts and automation.
