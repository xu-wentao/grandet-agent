package application_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/domain"
	"github.com/xu-wentao/grandet-agent/internal/testkit"
)

func TestWorkspaceInitializerUsesPorts(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	var directories, files, migrations, versions int
	initializer := application.NewWorkspaceInitializer(
		testkit.WorkspaceFilesystem{
			MkdirAllFunc:  func(string) error { directories++; return nil },
			WriteFileFunc: func(string, string, bool) (bool, error) { files++; return true, nil },
		},
		testkit.WorkspaceDatabase{
			MigrateFunc:        func(string) error { migrations++; return nil },
			RecordVersionsFunc: func(string, map[string]string) error { versions++; return nil },
		},
		testkit.FixedClock{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		testkit.FixedIDGenerator{ID: "test-id"},
	)

	result, err := initializer.Initialize(application.InitOptions{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if directories != len(result.Plan.Directories) || files != 5 || migrations != 1 || versions != 1 {
		t.Fatalf("calls: directories=%d files=%d migrations=%d versions=%d", directories, files, migrations, versions)
	}
}

func TestWorkspaceInitializerClassifiesStorageFailures(t *testing.T) {
	cause := errors.New("disk unavailable")
	initializer := application.NewWorkspaceInitializer(
		testkit.WorkspaceFilesystem{MkdirAllFunc: func(string) error { return cause }},
		testkit.WorkspaceDatabase{},
		testkit.FixedClock{},
		testkit.FixedIDGenerator{},
	)

	_, err := initializer.Initialize(application.InitOptions{Home: filepath.Join(t.TempDir(), ".grandet")})
	if !errors.Is(err, cause) {
		t.Fatalf("cause was not preserved: %v", err)
	}
	normalized, ok := domain.AsError(err)
	if !ok || normalized.Code != domain.CodePersistenceFailure {
		t.Fatalf("error = %#v, typed = %t", normalized, ok)
	}
}
