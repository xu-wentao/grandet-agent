package conformance

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/xu-wentao/grandet-agent/"

func TestDependencyBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, pkg := range []string{"internal/domain", "internal/application"} {
		files, err := filepath.Glob(filepath.Join(root, pkg, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, spec := range parsed.Imports {
				path := strings.Trim(spec.Path.Value, "\"")
				if forbidden(pkg, path) {
					t.Errorf("%s must not import %q", pkg, path)
				}
			}
		}
	}
}

func forbidden(pkg, path string) bool {
	if pkg == "internal/application" {
		return strings.HasPrefix(path, module+"internal/cli") || strings.HasPrefix(path, module+"internal/infrastructure")
	}
	if strings.HasPrefix(path, module) || strings.Contains(path, ".") {
		return true
	}
	return path == "database/sql" || path == "os" || path == "io/fs" || path == "path/filepath"
}

func TestForbiddenDependencies(t *testing.T) {
	for _, test := range []struct {
		pkg, path string
		want      bool
	}{
		{"internal/domain", "database/sql", true},
		{"internal/domain", module + "internal/infrastructure", true},
		{"internal/domain", "github.com/provider/sdk", true},
		{"internal/domain", "time", false},
		{"internal/application", module + "internal/infrastructure", true},
		{"internal/application", module + "internal/domain", false},
	} {
		if got := forbidden(test.pkg, test.path); got != test.want {
			t.Errorf("forbidden(%q, %q) = %t, want %t", test.pkg, test.path, got, test.want)
		}
	}
}
