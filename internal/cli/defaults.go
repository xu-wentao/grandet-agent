package cli

const defaultConfigYAML = `version: v1

strategy:
  name: stingy
  objective: minimize_cost_per_accepted_task
  cost_first: true
  allow_quality_escalation: true
  allow_reliability_fallback: true
  max_quality_escalation_depth: 3
  max_reliability_fallback_depth: 2
  max_retry_per_model: 1
  default_quality_floor: low_medium
  learn_from_feedback: true

baseline:
  enabled: true
  model: openai/gpt-5.5
  note: Used for savings analysis only. It is not called unless explicitly requested.

runtime:
  database: ~/.grandet/grandet.db
  trace_dir: ~/.grandet/traces
  cache_dir: ~/.grandet/cache
  default_timeout_seconds: 120
  stream: true

privacy:
  allow_shadow_eval_for_user_tasks: false
  require_confirm_before_using_free_models: false
  redact_before_eval: true

budget:
  daily_limit_usd: 1.00
  task_default_limit_usd: 0.05
  shadow_eval_daily_limit_usd: 0.20

cost_accounting:
  track_raw_call_cost: true
  track_task_total_cost: true
  track_accepted_task_cost: true
  track_wasted_cost: true
  track_fallback_tax: true

free_models:
  allow_for_real_tasks: true
  require_smoke_test: true
  require_task_bucket_profile: true
  max_consecutive_failures: 3
  auto_degrade_on_rate_limit: true

feedback:
  ask_after_run: false
  default_if_skipped: neutral

shadow_eval:
  enabled: false
  max_daily_budget_usd: 0.20
  prefer_free_models: true
  redact_before_eval: true
`

const defaultProvidersYAML = `providers:
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

const defaultModelsYAML = `models:
  - id: openrouter/qwen/qwen3-coder-free
    provider: openrouter
    name: qwen3-coder-free
    enabled: true
    is_free: true
    free_state: FREE_DISCOVERED
    capabilities:
      - coding_simple
      - coding_medium
      - chinese
      - summarization
    quality_hint: medium

  - id: deepseek/deepseek-chat
    provider: deepseek
    name: deepseek-chat
    enabled: false
    is_free: false
    capabilities:
      - chat
      - coding_simple
      - summarization
      - reasoning_light

  - id: openai/gpt-5.4-mini
    provider: openai
    name: gpt-5.4-mini
    enabled: false
    is_free: false
    capabilities:
      - coding_medium
      - reasoning
      - tool_calling
      - json_output
`

const defaultUserProfileYAML = `user:
  id: local
  profile_version: v1

preferences:
  cost_sensitivity: 0.92
  quality_sensitivity: 0.58
  latency_sensitivity: 0.30
  default_acceptance_threshold: 0.62

task_tolerance:
  coding_simple_patch:
    min_success_probability: 0.78
    reject_rate_7d: 0.00
    accept_rate_7d: 0.00
    preferred_price_quantile: 0.20
  summarization_short:
    min_success_probability: 0.60
    reject_rate_7d: 0.00
    accept_rate_7d: 0.00
    preferred_price_quantile: 0.05
  extraction_json:
    min_success_probability: 0.72
    reject_rate_7d: 0.00
    accept_rate_7d: 0.00
    preferred_price_quantile: 0.10
`
