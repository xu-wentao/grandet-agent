package application

import (
	"fmt"
	"path/filepath"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type WorkspaceFilesystem interface {
	MkdirAll(path string) error
	WriteFile(path, content string, force bool) (created bool, err error)
}

type WorkspaceDatabase interface {
	Migrate(path string) error
	RecordVersions(path string, versions map[string]string) error
}

type InitOptions struct {
	Home   string
	DryRun bool
	Force  bool
}

type InitPlan struct {
	Directories []string
	Files       []string
}

type InitResult struct {
	Plan    InitPlan
	Created []string
}

type WorkspaceInitializer struct {
	filesystem WorkspaceFilesystem
	database   WorkspaceDatabase
	clock      domain.Clock
	ids        domain.IDGenerator
}

func NewWorkspaceInitializer(filesystem WorkspaceFilesystem, database WorkspaceDatabase, clock domain.Clock, ids domain.IDGenerator) WorkspaceInitializer {
	return WorkspaceInitializer{filesystem: filesystem, database: database, clock: clock, ids: ids}
}

func (s WorkspaceInitializer) Initialize(options InitOptions) (InitResult, error) {
	plan := workspacePlan(options.Home)
	if options.DryRun {
		return InitResult{Plan: plan}, nil
	}
	for _, path := range plan.Directories {
		if err := s.filesystem.MkdirAll(path); err != nil {
			return InitResult{}, fmt.Errorf("create dir %s: %w", path, err)
		}
	}
	created := make([]string, 0, len(defaultFiles(options.Home)))
	for path, content := range defaultFiles(options.Home) {
		wrote, err := s.filesystem.WriteFile(path, content, options.Force)
		if err != nil {
			return InitResult{}, fmt.Errorf("write file %s: %w", path, err)
		}
		if wrote {
			created = append(created, path)
		}
	}
	databasePath := filepath.Join(options.Home, "grandet.db")
	if err := s.database.Migrate(databasePath); err != nil {
		return InitResult{}, fmt.Errorf("migrate database: %w", err)
	}
	if err := s.database.RecordVersions(databasePath, workspaceVersions()); err != nil {
		return InitResult{}, fmt.Errorf("record workspace versions: %w", err)
	}
	return InitResult{Plan: plan, Created: created}, nil
}

func workspacePlan(home string) InitPlan {
	return InitPlan{Directories: []string{home, filepath.Join(home, "logs"), filepath.Join(home, "traces"), filepath.Join(home, "cache"), filepath.Join(home, "policies"), filepath.Join(home, "evals"), filepath.Join(home, "evals", "golden"), filepath.Join(home, "evals", "regression"), filepath.Join(home, "evals", "safety")}, Files: []string{filepath.Join(home, "config.yaml"), filepath.Join(home, "providers.yaml"), filepath.Join(home, "models.yaml"), filepath.Join(home, "user-profile.yaml"), filepath.Join(home, "policies", "stingy-v1.yaml"), filepath.Join(home, "grandet.db")}}
}

func defaultFiles(home string) map[string]string {
	return map[string]string{filepath.Join(home, "config.yaml"): defaultConfigYAML, filepath.Join(home, "providers.yaml"): defaultProvidersYAML, filepath.Join(home, "models.yaml"): defaultModelsYAML, filepath.Join(home, "user-profile.yaml"): defaultUserProfileYAML, filepath.Join(home, "policies", "stingy-v1.yaml"): defaultPolicyYAML}
}

func workspaceVersions() map[string]string {
	return map[string]string{"config": "1", "providers": "1", "models": "1", "user-profile": "1", "policy": "1"}
}
