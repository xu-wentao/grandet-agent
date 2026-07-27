package architecture_test

import (
	"go/parser"
	"go/token"
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
		for _, path := range packageImports(t, filepath.Join(root, "internal", name)) {
			if forbidden(path) {
				t.Errorf("internal/%s must not import %q", name, path)
			}
		}
	}
}

func TestForbiddenDomainImports(t *testing.T) {
	for _, path := range []string{"database/sql", "os", "io/fs", "path/filepath", "modernc.org/sqlite", module + "/internal/cli"} {
		if !forbiddenDomainImport(path) {
			t.Errorf("%q must be forbidden in domain", path)
		}
	}
}

func forbiddenDomainImport(path string) bool {
	return path == "database/sql" || path == "os" || path == "io/fs" || path == "path/filepath" || strings.HasPrefix(path, "modernc.org/") || strings.HasPrefix(path, module+"/internal/")
}

func forbiddenCLIImport(path string) bool {
	if !strings.HasPrefix(path, module+"/internal/") {
		return false
	}
	return path != module+"/internal/application" && path != module+"/internal/infrastructure"
}

func packageImports(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var imports []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imports = append(imports, strings.Trim(spec.Path.Value, "\""))
		}
	}
	return imports
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate conformance test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
