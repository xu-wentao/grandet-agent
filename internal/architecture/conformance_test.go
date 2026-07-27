package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/xu-wentao/grandet-agent/internal/"

func TestArchitectureConformance(t *testing.T) {
	for _, rule := range []struct {
		directory    string
		forbidden    func(string) bool
		includeTests bool
	}{
		{filepath.Join("..", "domain"), forbiddenDomainImport, true},
		{filepath.Join("..", "application"), forbiddenApplicationImport, false},
		{filepath.Join("..", "cli"), forbiddenCLIImport, false},
	} {
		violations, err := forbiddenImports(rule.directory, rule.forbidden, rule.includeTests)
		if err != nil {
			t.Fatal(err)
		}
		for _, violation := range violations {
			t.Error(violation)
		}
	}
}

func TestDomainDependencyRuleRejectsForbiddenImports(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "forbidden.go")
	if err := os.WriteFile(filename, []byte("package domain\n\nimport \"os\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, err := forbiddenImports(directory, forbiddenDomainImport, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || !strings.Contains(violations[0], `"os"`) {
		t.Fatalf("forbidden import was not reported: %#v", violations)
	}
}

func forbiddenImports(directory string, forbidden func(string) bool, includeTests bool) ([]string, error) {
	packages, err := parser.ParseDir(token.NewFileSet(), directory, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, pkg := range packages {
		for filename, file := range pkg.Files {
			if !includeTests && strings.HasSuffix(filename, "_test.go") {
				continue
			}
			for _, spec := range file.Imports {
				importPath, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return nil, err
				}
				if forbidden(importPath) {
					violations = append(violations, filename+" imports forbidden dependency "+strconv.Quote(importPath))
				}
			}
		}
	}
	return violations, nil
}

func forbiddenDomainImport(importPath string) bool {
	if strings.HasPrefix(importPath, modulePath) {
		return importPath != modulePath+"domain"
	}
	return strings.Contains(strings.Split(importPath, "/")[0], ".") || hasPackagePrefix(importPath, "database/sql", "io/fs", "net", "os", "path/filepath")
}

func forbiddenApplicationImport(importPath string) bool {
	return importPath == modulePath+"cli" || importPath == modulePath+"infrastructure"
}

func forbiddenCLIImport(importPath string) bool {
	return importPath == modulePath+"domain"
}

func hasPackagePrefix(importPath string, packages ...string) bool {
	for _, pkg := range packages {
		if importPath == pkg || strings.HasPrefix(importPath, pkg+"/") {
			return true
		}
	}
	return false
}
