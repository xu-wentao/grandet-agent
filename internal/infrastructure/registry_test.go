package infrastructure

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type registryClock struct{ now time.Time }

func (c *registryClock) Now() time.Time { return c.now }

func TestRegistryPreservesPriceHistoryAndManualProfiles(t *testing.T) {
	clock := &registryClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	path := filepath.Join(t.TempDir(), "grandet.db")
	if err := NewSQLiteMigrator(clock).Migrate(path); err != nil {
		t.Fatal(err)
	}
	modelsPath := filepath.Join(filepath.Dir(path), "models.yaml")
	if err := os.WriteFile(modelsPath, []byte(`models:
  - id: local/demo
    provider: local
    upstream_name: demo
    enabled: true
    is_free: false
    lifecycle_state: ACTIVE
execution_profiles:
  - id: demo-no-thinking
    model: local/demo
    enabled: true
    reasoning:
      mode: disabled
  - id: demo-reasoning
    model: local/demo
    enabled: true
    reasoning:
      mode: high
`), 0o600); err != nil {
		t.Fatal(err)
	}
	registry := NewSQLiteRegistry(path, clock)
	config := application.ProviderConfig{Name: "local", Type: "openai_compatible", BaseURL: "https://example.test/v1", Enabled: true}
	if err := registry.UpsertProviders(context.Background(), []application.ProviderConfig{config}); err != nil {
		t.Fatal(err)
	}
	if err := registry.ImportManualProfiles(context.Background(), modelsPath); err != nil {
		t.Fatal(err)
	}
	if profiles, err := registry.EligibleExecutionProfiles(context.Background(), false); err != nil || len(profiles) != 0 {
		t.Fatalf("unknown paid profiles = %#v, %v", profiles, err)
	}
	price := func(value float64) *float64 { return &value }
	sync := func(input float64) {
		t.Helper()
		if err := registry.Sync(context.Background(), config, []domain.ProviderModel{{ID: "demo", Price: &domain.ModelPrice{InputPerMillion: price(input), OutputPerMillion: price(2), Source: "provider_sync"}}}); err != nil {
			t.Fatal(err)
		}
	}
	sync(1)
	sync(1)
	profiles, err := registry.EligibleExecutionProfiles(context.Background(), false)
	if err != nil || len(profiles) != 2 || profiles[0].ReasoningMode != "disabled" || profiles[1].ReasoningMode != "high" {
		t.Fatalf("eligible profiles = %#v, %v", profiles, err)
	}
	if err := registry.UpsertProviders(context.Background(), []application.ProviderConfig{{Name: "local", Type: "openai_compatible", BaseURL: "https://example.test/v1", Enabled: false}}); err != nil {
		t.Fatal(err)
	}
	if profiles, err := registry.EligibleExecutionProfiles(context.Background(), false); err != nil || len(profiles) != 0 {
		t.Fatalf("disabled provider profiles = %#v, %v", profiles, err)
	}
	if err := registry.UpsertProviders(context.Background(), []application.ProviderConfig{config}); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE models SET enabled = 0 WHERE id = 'local/demo'`); err != nil {
		t.Fatal(err)
	}
	if profiles, err := registry.EligibleExecutionProfiles(context.Background(), false); err != nil || len(profiles) != 0 {
		t.Fatalf("disabled model profiles = %#v, %v", profiles, err)
	}
	clock.now = clock.now.Add(time.Hour)
	sync(3)
	var prices, current int
	if err := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(effective_to IS NULL), 0) FROM model_prices WHERE model_id = 'local/demo'`).Scan(&prices, &current); err != nil {
		t.Fatal(err)
	}
	if prices != 2 || current != 1 {
		t.Fatalf("price history = %d rows, %d current; want 2, 1", prices, current)
	}
	clock.now = clock.now.Add(time.Hour)
	if err := registry.Sync(context.Background(), config, []domain.ProviderModel{{ID: "demo"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COALESCE(SUM(effective_to IS NULL), 0) FROM model_prices WHERE model_id = 'local/demo'`).Scan(&current); err != nil {
		t.Fatal(err)
	}
	if current != 0 {
		t.Fatalf("current prices after unknown sync = %d; want 0", current)
	}
	models, err := registry.ListModels(context.Background())
	if err != nil || len(models) != 1 || models[0].Enabled || models[0].LifecycleState != "ACTIVE" || models[0].PriceKnown {
		t.Fatalf("manual model override was erased: %#v, %v", models, err)
	}
}

func TestRegistrySyncMarksOmittedProviderModelsUnavailable(t *testing.T) {
	clock := &registryClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	path := filepath.Join(t.TempDir(), "grandet.db")
	if err := NewSQLiteMigrator(clock).Migrate(path); err != nil {
		t.Fatal(err)
	}
	registry := NewSQLiteRegistry(path, clock)
	config := application.ProviderConfig{Name: "local", Type: "openai_compatible", BaseURL: "https://example.test/v1", Enabled: true}
	if err := registry.UpsertProviders(context.Background(), []application.ProviderConfig{config}); err != nil {
		t.Fatal(err)
	}
	price := func(value float64) *float64 { return &value }
	if err := registry.Sync(context.Background(), config, []domain.ProviderModel{{ID: "gone", Price: &domain.ModelPrice{InputPerMillion: price(1), OutputPerMillion: price(2), Source: "provider_sync"}}}); err != nil {
		t.Fatal(err)
	}
	modelsPath := filepath.Join(filepath.Dir(path), "models.yaml")
	if err := os.WriteFile(modelsPath, []byte(`models:
  - id: local/manual-only
    provider: local
    upstream_name: manual-only
    enabled: true
    is_free: true
    lifecycle_state: ACTIVE
execution_profiles:
  - id: gone-profile
    model: local/gone
    enabled: true
  - id: manual-profile
    model: local/manual-only
    enabled: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := registry.ImportManualProfiles(context.Background(), modelsPath); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetModelState(context.Background(), "local/gone", "ACTIVE"); err != nil {
		t.Fatal(err)
	}
	if profiles, err := registry.EligibleExecutionProfiles(context.Background(), false); err != nil || len(profiles) != 2 {
		t.Fatalf("eligible profiles before omission = %#v, %v", profiles, err)
	}

	clock.now = clock.now.Add(time.Hour)
	if err := registry.Sync(context.Background(), config, nil); err != nil {
		t.Fatal(err)
	}

	models, err := registry.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ID != "local/gone" || models[0].Enabled || models[0].LifecycleState != "UNAVAILABLE" || models[0].PriceKnown {
		t.Fatalf("omitted provider model = %#v", models)
	}
	if models[1].ID != "local/manual-only" || !models[1].Enabled || models[1].LifecycleState != "ACTIVE" {
		t.Fatalf("manual-only model = %#v", models)
	}
	profiles, err := registry.EligibleExecutionProfiles(context.Background(), false)
	if err != nil || len(profiles) != 1 || profiles[0].ID != "manual-profile" {
		t.Fatalf("eligible profiles after omission = %#v, %v", profiles, err)
	}
}
