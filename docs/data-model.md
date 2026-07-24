# GrandetAgent Data Model

## 1. Purpose

GrandetAgent needs enough local data to answer four questions:

1. What happened during a trajectory?
2. Why did the router choose that execution profile?
3. What did the complete trajectory cost?
4. What evidence justifies changing future policy?

The first implementation uses SQLite. The schema should preserve immutable facts separately from derived profiles.

## 2. Data Categories

### Immutable facts

- trajectory creation
- domain events
- routing decisions
- model calls
- tool calls
- validation results
- cost ledger entries
- user feedback
- policy activation events

### Derived state

- model execution profile statistics
- user tolerance profiles
- provider health summaries
- task-distribution summaries
- policy recommendations

Derived state can be rebuilt from facts when needed.

## 3. Core Entity Relationships

```text
Session
  -> many Trajectories

Trajectory
  -> many Tasks
  -> many Steps
  -> many Routing Decisions
  -> many Domain Events
  -> many Cost Entries
  -> many Feedback Events

Task
  -> one Task Profile
  -> many Steps

Step
  -> zero or one Model Call
  -> zero or one Tool Call
  -> many Validation Results

Routing Decision
  -> one Policy Version
  -> one selected Execution Profile
  -> many candidate snapshots

Execution Profile
  -> one Provider
  -> one Model
  -> many task-family performance profiles
```

## 4. Tables

### 4.1 schema_versions

```sql
CREATE TABLE schema_versions (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);
```

`workspace_versions` records the schema versions of generated configuration and policy defaults:

```sql
CREATE TABLE workspace_versions (
  name TEXT PRIMARY KEY,
  version TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

### 4.2 sessions

```sql
CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  active_execution_profile_id TEXT,
  policy_version TEXT NOT NULL,
  context_fingerprint TEXT,
  estimated_cache_state TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

`estimated_cache_state` is advisory. It must not claim provider cache certainty when unavailable.

### 4.3 trajectories

```sql
CREATE TABLE trajectories (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  parent_trajectory_id TEXT,
  mode TEXT NOT NULL,
  status TEXT NOT NULL,
  prompt_hash TEXT NOT NULL,
  active_policy_version TEXT NOT NULL,
  command_budget_usd REAL,
  validated_success INTEGER,
  user_accepted INTEGER,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  FOREIGN KEY(session_id) REFERENCES sessions(id)
);
```

`mode` values:

```text
live
replay
shadow
golden_eval
```

### 4.4 trajectory_events

```sql
CREATE TABLE trajectory_events (
  id TEXT PRIMARY KEY,
  trajectory_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  event_version INTEGER NOT NULL,
  payload_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(trajectory_id) REFERENCES trajectories(id)
);
```

Events preserve the coordination history and allow replay.

### 4.5 tasks

```sql
CREATE TABLE tasks (
  id TEXT PRIMARY KEY,
  trajectory_id TEXT NOT NULL,
  parent_task_id TEXT,
  status TEXT NOT NULL,
  task_family TEXT NOT NULL,
  difficulty INTEGER NOT NULL,
  domain TEXT,
  risk_level TEXT,
  seriousness TEXT,
  context_size TEXT,
  verification_mode TEXT,
  analyzer_confidence REAL,
  created_at TEXT NOT NULL,
  completed_at TEXT,
  FOREIGN KEY(trajectory_id) REFERENCES trajectories(id)
);
```

### 4.6 task_features

```sql
CREATE TABLE task_features (
  task_id TEXT PRIMARY KEY,
  hard_constraints_json TEXT NOT NULL,
  semantic_features_json TEXT,
  required_tools_json TEXT,
  required_modalities_json TEXT,
  expected_input_tokens INTEGER,
  expected_output_tokens INTEGER,
  estimated_context_replay_tokens INTEGER,
  FOREIGN KEY(task_id) REFERENCES tasks(id)
);
```

### 4.7 steps

```sql
CREATE TABLE steps (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  sequence_no INTEGER NOT NULL,
  step_type TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT,
  completed_at TEXT,
  FOREIGN KEY(task_id) REFERENCES tasks(id)
);
```

`step_type` examples:

```text
local_operation
model_call
tool_call
validation
repair
judge
```

### 4.8 providers

```sql
CREATE TABLE providers (
  id TEXT PRIMARY KEY,
  provider_type TEXT NOT NULL,
  base_url TEXT,
  enabled INTEGER NOT NULL,
  config_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

Secrets are never stored directly in `config_json`; environment-variable references are stored instead.

### 4.9 models

```sql
CREATE TABLE models (
  id TEXT PRIMARY KEY,
  provider_id TEXT NOT NULL,
  upstream_model_name TEXT NOT NULL,
  lifecycle_state TEXT NOT NULL,
  is_free INTEGER NOT NULL,
  context_window INTEGER,
  capability_json TEXT NOT NULL,
  public_profile_json TEXT,
  discovered_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(provider_id) REFERENCES providers(id)
);
```

### 4.10 execution_profiles

```sql
CREATE TABLE execution_profiles (
  id TEXT PRIMARY KEY,
  model_id TEXT NOT NULL,
  enabled INTEGER NOT NULL,
  reasoning_mode TEXT,
  max_output_tokens INTEGER,
  temperature REAL,
  tool_calling INTEGER,
  profile_config_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(model_id) REFERENCES models(id)
);
```

### 4.11 model_prices

```sql
CREATE TABLE model_prices (
  id TEXT PRIMARY KEY,
  execution_profile_id TEXT NOT NULL,
  currency TEXT NOT NULL,
  input_per_million REAL,
  cached_input_per_million REAL,
  output_per_million REAL,
  reasoning_per_million REAL,
  effective_from TEXT NOT NULL,
  effective_to TEXT,
  source TEXT NOT NULL,
  FOREIGN KEY(execution_profile_id) REFERENCES execution_profiles(id)
);
```

Price history is preserved so historical trajectories remain explainable.

### 4.12 routing_policies

```sql
CREATE TABLE routing_policies (
  version TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  parent_version TEXT,
  policy_yaml TEXT NOT NULL,
  evidence_summary_json TEXT,
  validation_report_json TEXT,
  rollback_target_version TEXT,
  created_at TEXT NOT NULL,
  activated_at TEXT
);
```

### 4.13 routing_decisions

```sql
CREATE TABLE routing_decisions (
  id TEXT PRIMARY KEY,
  trajectory_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  policy_version TEXT NOT NULL,
  selected_execution_profile_id TEXT NOT NULL,
  selected_reason TEXT NOT NULL,
  estimated_trajectory_cost_usd REAL,
  estimated_success_probability REAL,
  estimated_latency_ms INTEGER,
  switch_penalty_usd REAL,
  context_replay_cost_usd REAL,
  uncertainty REAL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(trajectory_id) REFERENCES trajectories(id),
  FOREIGN KEY(task_id) REFERENCES tasks(id),
  FOREIGN KEY(policy_version) REFERENCES routing_policies(version),
  FOREIGN KEY(selected_execution_profile_id) REFERENCES execution_profiles(id)
);
```

### 4.14 routing_candidates

```sql
CREATE TABLE routing_candidates (
  id TEXT PRIMARY KEY,
  routing_decision_id TEXT NOT NULL,
  execution_profile_id TEXT NOT NULL,
  candidate_rank INTEGER,
  eligibility_status TEXT NOT NULL,
  filter_reason TEXT,
  estimated_cost_usd REAL,
  estimated_success_probability REAL,
  estimated_latency_ms INTEGER,
  estimated_fallback_tax_usd REAL,
  estimated_switch_penalty_usd REAL,
  feature_json TEXT,
  FOREIGN KEY(routing_decision_id) REFERENCES routing_decisions(id),
  FOREIGN KEY(execution_profile_id) REFERENCES execution_profiles(id)
);
```

Rejected candidates are retained for explainability.

### 4.15 model_calls

```sql
CREATE TABLE model_calls (
  id TEXT PRIMARY KEY,
  trajectory_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  step_id TEXT NOT NULL,
  execution_profile_id TEXT NOT NULL,
  call_role TEXT NOT NULL,
  status TEXT NOT NULL,
  provider_request_id TEXT,
  input_tokens INTEGER,
  cached_input_tokens INTEGER,
  output_tokens INTEGER,
  reasoning_tokens INTEGER,
  ttft_ms INTEGER,
  total_latency_ms INTEGER,
  normalized_error_type TEXT,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  FOREIGN KEY(execution_profile_id) REFERENCES execution_profiles(id)
);
```

`call_role` examples:

```text
primary
classifier
repair
judge
quality_escalation
reliability_fallback
shadow
```

### 4.16 tool_calls

```sql
CREATE TABLE tool_calls (
  id TEXT PRIMARY KEY,
  trajectory_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  step_id TEXT NOT NULL,
  tool_name TEXT NOT NULL,
  status TEXT NOT NULL,
  arguments_hash TEXT,
  latency_ms INTEGER,
  cost_usd REAL,
  error_type TEXT,
  created_at TEXT NOT NULL
);
```

### 4.17 validation_results

```sql
CREATE TABLE validation_results (
  id TEXT PRIMARY KEY,
  trajectory_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  step_id TEXT,
  validator_type TEXT NOT NULL,
  status TEXT NOT NULL,
  score REAL,
  details_json TEXT,
  created_at TEXT NOT NULL
);
```

`status` values:

```text
PASS
REPAIRABLE_FAILURE
INSUFFICIENT_CONTEXT
QUALITY_FAILURE
SAFETY_FAILURE
```

### 4.18 fallback_events

```sql
CREATE TABLE fallback_events (
  id TEXT PRIMARY KEY,
  trajectory_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  from_execution_profile_id TEXT,
  to_execution_profile_id TEXT,
  fallback_type TEXT NOT NULL,
  reason TEXT NOT NULL,
  context_replay_tokens INTEGER,
  additional_cost_usd REAL,
  created_at TEXT NOT NULL
);
```

`fallback_type` values:

```text
quality_escalation
reliability_fallback
budget_fallback
```

### 4.19 cost_ledger_entries

```sql
CREATE TABLE cost_ledger_entries (
  id TEXT PRIMARY KEY,
  trajectory_id TEXT NOT NULL,
  task_id TEXT,
  step_id TEXT,
  model_call_id TEXT,
  category TEXT NOT NULL,
  estimated_amount_usd REAL,
  actual_amount_usd REAL,
  source TEXT NOT NULL,
  metadata_json TEXT,
  created_at TEXT NOT NULL
);
```

Categories:

```text
routing
input
cached_input
output
reasoning
cache_write
context_replay
tool
validation
repair
judge
retry
quality_escalation
reliability_fallback
rejected_work
```

### 4.20 feedback_events

```sql
CREATE TABLE feedback_events (
  id TEXT PRIMARY KEY,
  trajectory_id TEXT NOT NULL,
  feedback_type TEXT NOT NULL,
  reason TEXT,
  rating INTEGER,
  explicit INTEGER NOT NULL,
  evidence_weight REAL NOT NULL,
  metadata_json TEXT,
  created_at TEXT NOT NULL
);
```

Feedback is append-only. Corrections create compensating events rather than rewriting history.

### 4.21 user_tolerance_profiles

```sql
CREATE TABLE user_tolerance_profiles (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  task_family TEXT NOT NULL,
  difficulty INTEGER,
  domain TEXT,
  min_success_probability REAL NOT NULL,
  cost_sensitivity REAL NOT NULL,
  latency_sensitivity REAL NOT NULL,
  sample_count INTEGER NOT NULL,
  evidence_window_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

### 4.22 execution_profile_performance

```sql
CREATE TABLE execution_profile_performance (
  id TEXT PRIMARY KEY,
  execution_profile_id TEXT NOT NULL,
  task_family TEXT NOT NULL,
  difficulty INTEGER,
  domain TEXT,
  success_rate REAL,
  accept_rate REAL,
  reject_rate REAL,
  reask_rate REAL,
  reliability_failure_rate REAL,
  quality_escalation_rate REAL,
  avg_raw_call_cost_usd REAL,
  avg_trajectory_cost_usd REAL,
  avg_accepted_trajectory_cost_usd REAL,
  avg_fallback_tax_usd REAL,
  avg_latency_ms REAL,
  sample_count INTEGER NOT NULL,
  confidence REAL NOT NULL,
  updated_at TEXT NOT NULL
);
```

### 4.23 eval_cases

```sql
CREATE TABLE eval_cases (
  id TEXT PRIMARY KEY,
  suite_name TEXT NOT NULL,
  task_family TEXT NOT NULL,
  difficulty INTEGER NOT NULL,
  domain TEXT,
  prompt TEXT NOT NULL,
  context_json TEXT,
  acceptance_criteria_json TEXT NOT NULL,
  validators_json TEXT NOT NULL,
  safety_requirements_json TEXT,
  enabled INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

### 4.24 eval_runs

```sql
CREATE TABLE eval_runs (
  id TEXT PRIMARY KEY,
  run_type TEXT NOT NULL,
  suite_name TEXT,
  source_trajectory_id TEXT,
  policy_version TEXT,
  status TEXT NOT NULL,
  budget_usd REAL,
  total_cost_usd REAL,
  started_at TEXT NOT NULL,
  completed_at TEXT
);
```

### 4.25 eval_results

```sql
CREATE TABLE eval_results (
  id TEXT PRIMARY KEY,
  eval_run_id TEXT NOT NULL,
  eval_case_id TEXT,
  execution_profile_id TEXT,
  trajectory_id TEXT,
  validation_pass INTEGER,
  quality_score REAL,
  cost_usd REAL,
  latency_ms INTEGER,
  result_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);
```

## 5. Derived Metrics

### 5.1 trajectory_total_cost

```sql
SELECT SUM(actual_amount_usd)
FROM cost_ledger_entries
WHERE trajectory_id = ?;
```

### 5.2 cost_per_successful_trajectory

```text
sum(cost of technically successful trajectories)
/
count(technically successful trajectories)
```

### 5.3 cost_per_accepted_trajectory

```text
sum(cost of accepted trajectories)
/
count(accepted trajectories)
```

For aggregate budget analysis, rejected trajectory cost should also be reported separately rather than silently excluded.

### 5.4 fallback_tax

```text
repair + retry + judge + quality escalation + reliability fallback + context replay
```

### 5.5 routing_overhead_ratio

```text
routing cost / total trajectory generation cost
```

## 6. Indexing

Recommended indexes:

```sql
CREATE INDEX idx_trajectories_session_started
  ON trajectories(session_id, started_at);

CREATE INDEX idx_tasks_family_difficulty
  ON tasks(task_family, difficulty, domain);

CREATE INDEX idx_calls_profile_started
  ON model_calls(execution_profile_id, started_at);

CREATE INDEX idx_feedback_trajectory
  ON feedback_events(trajectory_id, created_at);

CREATE INDEX idx_cost_trajectory
  ON cost_ledger_entries(trajectory_id, created_at);

CREATE INDEX idx_profile_performance_lookup
  ON execution_profile_performance(
    execution_profile_id,
    task_family,
    difficulty,
    domain
  );
```

## 7. Retention and Privacy

Configuration should support:

- prompt storage off by default or configurable
- prompt hashing for cost analytics
- redacted context snapshots
- trace retention limits
- deletion of raw content while retaining aggregate cost facts

Provider API keys are never persisted in SQLite.

## 8. Migration Philosophy

- migrations are ordered and immutable
- failed migrations stop startup
- derived profile tables may be rebuilt
- immutable event and cost facts must never be discarded by a normal migration
- schema version is included in diagnostic output

## 9. Repository Boundaries

Suggested repositories:

```go
SessionRepository
TrajectoryRepository
TaskRepository
ExecutionProfileRepository
PolicyRepository
RoutingDecisionRepository
CostLedgerRepository
FeedbackRepository
EvaluationRepository
```

Repositories expose domain types rather than SQL rows.

## 10. Why This Model Is Deliberately Detailed

A minimal schema with only `task`, `model`, and `cost` cannot support:

- trajectory economics
- policy replay
- session switching analysis
- separate quality and reliability statistics
- user-specific tolerance learning
- policy rollback evidence

GrandetAgent's value depends on traceable economics. The storage model must preserve enough facts to explain every dollar and every policy change.
