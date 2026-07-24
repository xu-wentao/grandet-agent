package application

const DefaultConfigYAML = `schema_version: v2

strategy:
  name: stingy
  objective: minimize_cost_per_accepted_trajectory
  active_policy: stingy-v1
  cost_first: true
  allow_quality_escalation: true
  allow_reliability_fallback: true
  allow_budget_fallback: true
  max_quality_escalation_depth: 3
  max_reliability_fallback_depth: 2
  max_retry_per_profile: 1
  learn_from_feedback: true
  auto_activate_learned_policy: false

router:
  max_latency_ms: 50
  max_cost_ratio: 0.01
  allow_llm_classifier: false
  min_expected_routing_value_usd: 0.0001
  uncertainty_margin: 0.10

session:
  model_affinity: true
  track_context_fingerprint: true
  switch_penalty_enabled: true
  prefer_reasoning_mode_change_before_model_switch: true

baseline:
  enabled: true
  execution_profile: openai-mini-default
  mode: estimated
  note: Used for savings analysis unless explicitly executed.

runtime:
  database: ~/.grandet/grandet.db
  policy_dir: ~/.grandet/policies
  trace_dir: ~/.grandet/traces
  cache_dir: ~/.grandet/cache
  default_timeout_seconds: 120
  stream: true

budget:
  daily_limit_usd: 1.00
  trajectory_default_limit_usd: 0.05
  shadow_eval_daily_limit_usd: 0.20

privacy:
  store_raw_prompts: false
  allow_shadow_eval_for_user_tasks: false
  require_confirm_before_using_free_models: false
  redact_before_eval: true

policy:
  require_static_validation: true
  require_golden_set_validation: true
  require_explicit_activation: true
  rollback_on_safety_failure: true
  quality_regression_threshold: 0.05
  reask_zscore_threshold: 2.0
`

const DefaultProvidersYAML = `schema_version: v1

providers:
  openrouter:
    type: openai_compatible
    base_url: https://openrouter.ai/api/v1
    api_key_env: OPENROUTER_API_KEY
    enabled: true

  litellm:
    type: openai_compatible
    base_url: http://localhost:4000/v1
    api_key_env: LITELLM_API_KEY
    enabled: false

  openai:
    type: openai_compatible
    base_url: https://api.openai.com/v1
    api_key_env: OPENAI_API_KEY
    enabled: false

  deepseek:
    type: openai_compatible
    base_url: https://api.deepseek.com/v1
    api_key_env: DEEPSEEK_API_KEY
    enabled: false

  local_ollama:
    type: ollama
    base_url: http://localhost:11434
    enabled: false
`

const DefaultModelsYAML = `schema_version: v1

models:
  - id: openrouter/qwen/qwen3-coder-free
    provider: openrouter
    upstream_name: qwen3-coder-free
    enabled: true
    is_free: true
    lifecycle_state: DISCOVERED
    context_window: 131072
    capabilities:
      - code_generation
      - debugging
      - documentation

  - id: deepseek/deepseek-chat
    provider: deepseek
    upstream_name: deepseek-chat
    enabled: false
    is_free: false
    lifecycle_state: ACTIVE
    capabilities:
      - general_qa
      - code_generation
      - debugging
      - summarization

  - id: openai/gpt-5.4-mini
    provider: openai
    upstream_name: gpt-5.4-mini
    enabled: false
    is_free: false
    lifecycle_state: ACTIVE
    capabilities:
      - code_generation
      - architecture_design
      - reasoning
      - tool_calling
      - json_output

execution_profiles:
  - id: qwen-free-no-thinking
    model: openrouter/qwen/qwen3-coder-free
    enabled: true
    reasoning:
      mode: disabled
    max_output_tokens: 1200
    temperature: 0.2
    tool_calling: false

  - id: deepseek-chat-default
    model: deepseek/deepseek-chat
    enabled: false
    reasoning:
      mode: disabled
    max_output_tokens: 2000
    temperature: 0.2
    tool_calling: true

  - id: openai-mini-default
    model: openai/gpt-5.4-mini
    enabled: false
    reasoning:
      mode: low
    max_output_tokens: 2400
    temperature: 0.2
    tool_calling: true
`

const DefaultUserProfileYAML = `schema_version: v1

user:
  id: local
  profile_version: v2

preferences:
  cost_sensitivity: 0.92
  quality_sensitivity: 0.58
  latency_sensitivity: 0.30
  default_acceptance_threshold: 0.62

feedback_weights:
  explicit_accept: 1.00
  explicit_reject: -1.00
  manual_replay: -0.70
  manual_model_override: -0.60
  implicit_reask: -0.40
  large_manual_edit: -0.30

task_tolerance:
  - task_family: code_generation
    difficulty: 2
    domain: general
    min_success_probability: 0.76
    preferred_price_quantile: 0.20
    sample_count: 0

  - task_family: debugging
    difficulty: 3
    domain: kubernetes
    min_success_probability: 0.82
    preferred_price_quantile: 0.25
    sample_count: 0
`

const DefaultPolicyYAML = `schema_version: stingy-v1

metadata:
  name: stingy
  version: stingy-v1
  status: DRAFT

objective:
  type: minimize_cost_per_accepted_trajectory
  cost_first: true

signals:
  hard_constraints:
    enabled: true
  local_rules:
    enabled: true
  semantic_classifier:
    enabled: false
  llm_classifier:
    enabled: false

routing:
  preserve_session_affinity: true
  account_for_context_replay: true
  max_router_latency_ms: 50
  max_router_cost_ratio: 0.01

constraints:
  min_success_probability: 0.62
  max_trajectory_cost_usd: 0.05

recovery:
  quality_escalation:
    enabled: true
    max_depth: 3
  reliability_fallback:
    enabled: true
    max_depth: 2
  budget_fallback:
    enabled: true

validation:
  deterministic_first: true
  allow_live_llm_judge: true

kill_switch:
  safety_failure: true
  quality_regression_threshold: 0.05
  reask_zscore_threshold: 2.0
  routing_overhead_ratio_threshold: 0.01
`
