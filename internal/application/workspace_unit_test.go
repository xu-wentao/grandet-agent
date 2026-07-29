package application_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/application"
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
