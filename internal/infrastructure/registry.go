package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type RegisteredModel struct {
	ID             string
	Provider       string
	UpstreamName   string
	Enabled        bool
	LifecycleState string
	IsFree         bool
	PriceKnown     bool
}

type SQLiteRegistry struct {
	path  string
	clock domain.Clock
}

func NewSQLiteRegistry(path string, clock domain.Clock) SQLiteRegistry {
	return SQLiteRegistry{path: path, clock: clock}
}

func (r SQLiteRegistry) UpsertProviders(ctx context.Context, configs []application.ProviderConfig) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	for _, config := range configs {
		metadata, err := json.Marshal(map[string]string{"api_key_env": config.APIKeyEnv})
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO providers(id, provider_type, base_url, enabled, source_metadata_json, updated_at) VALUES(?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET provider_type = excluded.provider_type, base_url = excluded.base_url, enabled = excluded.enabled, source_metadata_json = excluded.source_metadata_json, updated_at = excluded.updated_at`, config.Name, config.Type, config.BaseURL, config.Enabled, string(metadata), r.now()); err != nil {
			return fmt.Errorf("save provider %q: %w", config.Name, err)
		}
	}
	return nil
}

// ImportManualProfiles loads local profile configuration without replacing
// model lifecycle state maintained by the registry.
func (r SQLiteRegistry) ImportManualProfiles(ctx context.Context, path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read models: %w", err)
	}
	var document modelsFile
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("parse models: %w", err)
	}
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, model := range document.Models {
		if model.ID == "" || model.Provider == "" || model.UpstreamName == "" {
			tx.Rollback()
			return fmt.Errorf("manual model requires id, provider, and upstream_name")
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO providers(id, provider_type, base_url, enabled, source_metadata_json, updated_at) VALUES(?, 'configured', '', 1, '{}', ?)`, model.Provider, r.now()); err != nil {
			tx.Rollback()
			return err
		}
		capabilities, err := json.Marshal(capabilitiesFromNames(model.Capabilities))
		if err != nil {
			tx.Rollback()
			return err
		}
		state := model.LifecycleState
		if state == "" {
			state = "ACTIVE"
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO models(id, provider_id, upstream_model_name, enabled, lifecycle_state, is_free, context_window, capability_json, source_metadata_json, source, discovered_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, '{}', 'manual_config', ?, ?)`, model.ID, model.Provider, model.UpstreamName, model.Enabled, state, model.IsFree, model.ContextWindow, string(capabilities), r.now(), r.now()); err != nil {
			tx.Rollback()
			return err
		}
	}
	for _, profile := range document.ExecutionProfiles {
		if profile.ID == "" || profile.Model == "" {
			tx.Rollback()
			return fmt.Errorf("manual execution profile requires id and model")
		}
		config, err := json.Marshal(profile)
		if err != nil {
			tx.Rollback()
			return err
		}
		mode := profile.Reasoning.Mode
		if mode == "" {
			mode = "disabled"
		}
		retryPolicy := profile.RetryPolicy
		if retryPolicy == "" {
			retryPolicy = "none"
		}
		qualityTier := profile.QualityTier
		if qualityTier == "" {
			qualityTier = "unknown"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO execution_profiles(id, model_id, enabled, reasoning_mode, reasoning_effort, max_output_tokens, tool_calling, json_output, vision, retry_policy, quality_tier, profile_config_json, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET model_id = excluded.model_id, enabled = excluded.enabled, reasoning_mode = excluded.reasoning_mode, reasoning_effort = excluded.reasoning_effort, max_output_tokens = excluded.max_output_tokens, tool_calling = excluded.tool_calling, json_output = excluded.json_output, vision = excluded.vision, retry_policy = excluded.retry_policy, quality_tier = excluded.quality_tier, profile_config_json = excluded.profile_config_json, updated_at = excluded.updated_at`, profile.ID, profile.Model, profile.Enabled, mode, profile.Reasoning.Effort, profile.MaxOutputTokens, profile.ToolCalling, profile.JSONOutput, profile.Vision, retryPolicy, qualityTier, string(config), r.now(), r.now()); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r SQLiteRegistry) Sync(ctx context.Context, provider application.ProviderConfig, models []domain.ProviderModel) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, model := range models {
		if model.ID == "" {
			tx.Rollback()
			return fmt.Errorf("provider %q returned a model without an id", provider.Name)
		}
		capabilities, err := json.Marshal(model.Capabilities)
		if err != nil {
			tx.Rollback()
			return err
		}
		id := provider.Name + "/" + model.ID
		if _, err := tx.ExecContext(ctx, `INSERT INTO models(id, provider_id, upstream_model_name, enabled, lifecycle_state, is_free, context_window, capability_json, source_metadata_json, source, discovered_at, updated_at) VALUES(?, ?, ?, 0, 'DISCOVERED', ?, ?, ?, ?, 'provider_sync', ?, ?) ON CONFLICT(provider_id, upstream_model_name) DO UPDATE SET is_free = excluded.is_free, context_window = excluded.context_window, capability_json = excluded.capability_json, source_metadata_json = excluded.source_metadata_json, source = excluded.source, updated_at = excluded.updated_at`, id, provider.Name, model.ID, model.IsFree, model.ContextWindow, string(capabilities), model.SourceMetadata, r.now(), r.now()); err != nil {
			tx.Rollback()
			return err
		}
		if err := r.recordPrice(ctx, tx, provider.Name, model.ID, model.Price); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r SQLiteRegistry) recordPrice(ctx context.Context, tx *sql.Tx, providerID, upstreamName string, price *domain.ModelPrice) error {
	if price == nil {
		return nil
	}
	var modelID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM models WHERE provider_id = ? AND upstream_model_name = ?`, providerID, upstreamName).Scan(&modelID); err != nil {
		return err
	}
	var current struct {
		input, cached, output, reasoning sql.NullFloat64
		source                           string
	}
	err := tx.QueryRowContext(ctx, `SELECT input_per_million, cached_input_per_million, output_per_million, reasoning_per_million, source FROM model_prices WHERE model_id = ? AND effective_to IS NULL`, modelID).Scan(&current.input, &current.cached, &current.output, &current.reasoning, &current.source)
	if err == nil && samePrice(current, price) {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	now := r.now()
	if err == nil {
		if _, err := tx.ExecContext(ctx, `UPDATE model_prices SET effective_to = ? WHERE model_id = ? AND effective_to IS NULL`, now, modelID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO model_prices(model_id, input_per_million, cached_input_per_million, output_per_million, reasoning_per_million, effective_from, source) VALUES(?, ?, ?, ?, ?, ?, ?)`, modelID, price.InputPerMillion, price.CachedInputPerMillion, price.OutputPerMillion, price.ReasoningPerMillion, now, price.Source)
	return err
}

func samePrice(current struct {
	input, cached, output, reasoning sql.NullFloat64
	source                           string
}, price *domain.ModelPrice) bool {
	return sameFloat(current.input, price.InputPerMillion) && sameFloat(current.cached, price.CachedInputPerMillion) && sameFloat(current.output, price.OutputPerMillion) && sameFloat(current.reasoning, price.ReasoningPerMillion) && current.source == price.Source
}

func sameFloat(current sql.NullFloat64, next *float64) bool {
	return current.Valid == (next != nil) && (!current.Valid || current.Float64 == *next)
}

func (r SQLiteRegistry) ListModels(ctx context.Context) ([]RegisteredModel, error) {
	db, err := r.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `SELECT m.id, m.provider_id, m.upstream_model_name, m.enabled, m.lifecycle_state, m.is_free, EXISTS(SELECT 1 FROM model_prices p WHERE p.model_id = m.id AND p.effective_to IS NULL AND p.input_per_million IS NOT NULL AND p.output_per_million IS NOT NULL) FROM models m ORDER BY m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []RegisteredModel
	for rows.Next() {
		var model RegisteredModel
		if err := rows.Scan(&model.ID, &model.Provider, &model.UpstreamName, &model.Enabled, &model.LifecycleState, &model.IsFree, &model.PriceKnown); err != nil {
			return nil, err
		}
		result = append(result, model)
	}
	return result, rows.Err()
}

func (r SQLiteRegistry) SetModelState(ctx context.Context, id, state string) error {
	enabled := state == "ACTIVE"
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.ExecContext(ctx, `UPDATE models SET enabled = ?, lifecycle_state = ?, updated_at = ? WHERE id = ?`, enabled, state, r.now(), id)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return err
		}
		return fmt.Errorf("model %q is not registered", id)
	}
	return nil
}

func (r SQLiteRegistry) ListExecutionProfiles(ctx context.Context) ([]domain.ModelExecutionProfile, error) {
	return r.profiles(ctx, "")
}

func (r SQLiteRegistry) ExecutionProfile(ctx context.Context, id string) (domain.ModelExecutionProfile, error) {
	profiles, err := r.profiles(ctx, id)
	if err != nil {
		return domain.ModelExecutionProfile{}, err
	}
	if len(profiles) == 0 {
		return domain.ModelExecutionProfile{}, fmt.Errorf("execution profile %q is not registered", id)
	}
	return profiles[0], nil
}

func (r SQLiteRegistry) EligibleExecutionProfiles(ctx context.Context, allowUnknownPaid bool) ([]domain.ModelExecutionProfile, error) {
	profiles, err := r.ListExecutionProfiles(ctx)
	if err != nil {
		return nil, err
	}
	eligible := make([]domain.ModelExecutionProfile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.EligibleForAutomaticRouting(allowUnknownPaid) {
			eligible = append(eligible, profile)
		}
	}
	return eligible, nil
}

func (r SQLiteRegistry) profiles(ctx context.Context, id string) ([]domain.ModelExecutionProfile, error) {
	db, err := r.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	query := `SELECT e.id, m.provider_id, m.upstream_model_name, e.reasoning_mode, e.reasoning_effort, e.max_output_tokens, e.tool_calling, e.json_output, e.vision, e.retry_policy, e.quality_tier, e.enabled, m.lifecycle_state, m.is_free, EXISTS(SELECT 1 FROM model_prices p WHERE p.model_id = m.id AND p.effective_to IS NULL AND p.input_per_million IS NOT NULL AND p.output_per_million IS NOT NULL) FROM execution_profiles e JOIN models m ON m.id = e.model_id`
	args := []any{}
	if id != "" {
		query += ` WHERE e.id = ?`
		args = append(args, id)
	}
	query += ` ORDER BY e.id`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.ModelExecutionProfile
	for rows.Next() {
		var profile domain.ModelExecutionProfile
		if err := rows.Scan(&profile.ID, &profile.Provider, &profile.Model, &profile.ReasoningMode, &profile.ReasoningEffort, &profile.MaxOutputTokens, &profile.Capabilities.ToolCalling, &profile.Capabilities.JSONOutput, &profile.Capabilities.Vision, &profile.RetryPolicy, &profile.QualityTier, &profile.Enabled, &profile.LifecycleState, &profile.IsFree, &profile.PriceKnown); err != nil {
			return nil, err
		}
		result = append(result, profile)
	}
	return result, rows.Err()
}

func (r SQLiteRegistry) open() (*sql.DB, error) {
	db, err := sql.Open("sqlite", r.path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (r SQLiteRegistry) now() string { return r.clock.Now().UTC().Format(time.RFC3339Nano) }

func capabilitiesFromNames(names []string) domain.ModelCapabilities {
	return domain.ModelCapabilities{ToolCalling: contains(names, "tool_calling"), JSONOutput: contains(names, "json_output"), Vision: contains(names, "vision")}
}

var _ domain.ExecutionProfileReader = SQLiteRegistry{}
