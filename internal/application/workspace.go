package application

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

const (
	ConfigSchemaVersion = "v2"
	PolicySchemaVersion = "stingy-v1"
)

type FileSystem interface {
	MkdirAll(path string) error
	Exists(path string) (bool, error)
	WriteFile(path, content string) error
}

type WorkspaceMigrator interface {
	Migrate(ctx context.Context, databasePath string, versions WorkspaceVersions) error
}

type WorkspaceVersions struct {
	Config string
	Policy string
	At     string
}

type InitRequest struct {
	Home   string
	DryRun bool
	Force  bool
}

type InitResult struct {
	Directories []string
	Files       []string
	Created     []string
	Skipped     []string
}

type InitializeWorkspaceService struct {
	Files    FileSystem
	Migrator WorkspaceMigrator
	Clock    domain.Clock
}

func (s InitializeWorkspaceService) Initialize(ctx context.Context, request InitRequest) (InitResult, error) {
	if request.Home == "" {
		return InitResult{}, fmt.Errorf("workspace home is required")
	}
	if s.Files == nil || s.Migrator == nil || s.Clock == nil {
		return InitResult{}, fmt.Errorf("workspace service dependencies are required")
	}

	result := InitResult{Directories: workspaceDirectories(request.Home)}
	files := workspaceFiles(request.Home)
	for path := range files {
		result.Files = append(result.Files, path)
	}
	result.Files = append(result.Files, filepath.Join(request.Home, "grandet.db"))
	sort.Strings(result.Files)
	if request.DryRun {
		return result, nil
	}

	for _, path := range result.Directories {
		if err := s.Files.MkdirAll(path); err != nil {
			return InitResult{}, fmt.Errorf("create dir %s: %w", path, err)
		}
	}
	for path, content := range files {
		exists, err := s.Files.Exists(path)
		if err != nil {
			return InitResult{}, fmt.Errorf("check file %s: %w", path, err)
		}
		if exists && !request.Force {
			result.Skipped = append(result.Skipped, path)
			continue
		}
		if err := s.Files.WriteFile(path, content); err != nil {
			return InitResult{}, fmt.Errorf("write file %s: %w", path, err)
		}
		result.Created = append(result.Created, path)
	}
	if err := s.Migrator.Migrate(ctx, filepath.Join(request.Home, "grandet.db"), WorkspaceVersions{
		Config: ConfigSchemaVersion,
		Policy: PolicySchemaVersion,
		At:     s.Clock.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}); err != nil {
		return InitResult{}, fmt.Errorf("migrate database: %w", err)
	}
	return result, nil
}

func workspaceDirectories(home string) []string {
	return []string{
		home,
		filepath.Join(home, "logs"),
		filepath.Join(home, "traces"),
		filepath.Join(home, "cache"),
		filepath.Join(home, "policies"),
		filepath.Join(home, "evals"),
		filepath.Join(home, "evals", "golden"),
		filepath.Join(home, "evals", "regression"),
		filepath.Join(home, "evals", "safety"),
	}
}

func workspaceFiles(home string) map[string]string {
	return map[string]string{
		filepath.Join(home, "config.yaml"):                DefaultConfigYAML,
		filepath.Join(home, "providers.yaml"):             DefaultProvidersYAML,
		filepath.Join(home, "models.yaml"):                DefaultModelsYAML,
		filepath.Join(home, "user-profile.yaml"):          DefaultUserProfileYAML,
		filepath.Join(home, "policies", "stingy-v1.yaml"): DefaultPolicyYAML,
	}
}
