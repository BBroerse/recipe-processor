package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modulePrefix = "github.com/bbroerse/recipe-processor/internal/"

// TestInfrastructureDoesNotImportApplication verifies that packages inside
// internal/infrastructure/ never import internal/application. In clean
// architecture the dependency arrow points inward: infrastructure →
// domain. The application layer orchestrates domain objects, and
// infrastructure implements domain interfaces — there is no reason for
// infrastructure to depend on application directly.
func TestInfrastructureDoesNotImportApplication(t *testing.T) {
	root := findRepoRoot(t)
	infraDir := filepath.Join(root, "internal", "infrastructure")

	err := filepath.Walk(infraDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Only inspect non-test Go source files (test files may use
		// application types for integration wiring — that's acceptable).
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("failed to parse %s: %v", path, parseErr)
		}

		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(importPath, modulePrefix+"application") {
				relPath, _ := filepath.Rel(root, path)
				t.Errorf("architecture violation: %s imports %s — infrastructure must not depend on application",
					relPath, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking infrastructure dir: %v", err)
	}
}

// findRepoRoot walks up from the test file location to find the repository
// root (identified by the presence of go.mod).
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root (go.mod)")
		}
		dir = parent
	}
}
