package architecture_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const module = "github.com/xu-wentao/grandet-agent"

func TestPackageDependencies(t *testing.T) {
	root := repositoryRoot(t)
	rules := map[string]func(string) bool{
		"cli": forbiddenCLIImport,
		"application": func(path string) bool {
			return strings.HasPrefix(path, module+"/internal/cli") || strings.HasPrefix(path, module+"/internal/infrastructure")
		},
		"infrastructure": func(path string) bool { return strings.HasPrefix(path, module+"/internal/cli") },
		"domain":         forbiddenDomainImport,
	}

	for name, forbidden := range rules {
		imports, err := packageImports(filepath.Join(root, "internal", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range imports {
			if forbidden(path) {
				t.Errorf("internal/%s must not import %q", name, path)
			}
		}
	}
}

func TestForbiddenDomainImports(t *testing.T) {
	for _, path := range []string{"database/sql", "os", "io/fs", "path/filepath", "modernc.org/sqlite", "github.com/vendor/sdk", module + "/internal/cli"} {
		if !forbiddenDomainImport(path) {
			t.Errorf("%q must be forbidden in domain", path)
		}
	}
	if forbiddenDomainImport("fmt") {
		t.Error("stdlib imports must be allowed in domain")
	}
}

func forbiddenDomainImport(path string) bool {
	return path == "database/sql" || path == "os" || path == "io/fs" || path == "path/filepath" || strings.Contains(path, ".")
}

func forbiddenCLIImport(path string) bool {
	if !strings.HasPrefix(path, module+"/internal/") {
		return false
	}
	return path != module+"/internal/application" && path != module+"/internal/infrastructure"
}

func TestRepositoryRoot(t *testing.T) {
	if _, err := os.Stat(filepath.Join(repositoryRoot(t), "go.mod")); err != nil {
		t.Fatalf("repository root must contain go.mod: %v", err)
	}
}

func TestPackageImportsRejectsEmptyLayer(t *testing.T) {
	if _, err := packageImports(t.TempDir()); err == nil {
		t.Error("missing layer must fail conformance check")
	}
}

func packageImports(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no Go files in expected layer %s", dir)
	}
	var imports []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		for _, spec := range file.Imports {
			imports = append(imports, strings.Trim(spec.Path.Value, "\""))
		}
	}
	return imports, nil
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate conformance test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
