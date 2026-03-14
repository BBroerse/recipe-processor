package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const moduleBase = "github.com/bbroerse/recipe-processor/internal"

// layerRule defines which internal sub-packages a layer is allowed to import.
type layerRule struct {
	name    string   // human-readable layer name
	dir     string   // directory relative to internal/ (e.g. "domain")
	allowed []string // allowed internal sub-package prefixes (e.g. "domain", "shared")
}

// rules encodes the strict dependency direction of the clean architecture.
//
// Domain         -> nothing internal
// Application    -> domain only
// Shared         -> domain only (must NOT import application or infrastructure)
// Infrastructure -> domain and shared only (must NOT import application)
var rules = []layerRule{
	{
		name:    "domain",
		dir:     "domain",
		allowed: nil, // no internal imports allowed
	},
	{
		name:    "application",
		dir:     "application",
		allowed: []string{"domain"},
	},
	{
		name:    "shared",
		dir:     "shared",
		allowed: []string{"domain"},
	},
	{
		name:    "infrastructure",
		dir:     "infrastructure",
		allowed: []string{"domain", "shared"},
	},
}

func TestDependencyDirection(t *testing.T) {
	internalDir := findInternalDir(t)

	for _, rule := range rules {
		rule := rule
		t.Run(rule.name, func(t *testing.T) {
			layerDir := filepath.Join(internalDir, rule.dir)
			if _, err := os.Stat(layerDir); os.IsNotExist(err) {
				t.Skipf("layer directory %s does not exist", layerDir)
				return
			}

			violations := checkLayer(t, internalDir, layerDir, rule)
			for _, v := range violations {
				t.Errorf("VIOLATION: %s imports %s \u2014 %s must not import %s",
					v.file, v.importPath, rule.name, v.violatedPkg)
			}
		})
	}
}

type violation struct {
	file        string // relative path from internal/
	importPath  string // the forbidden import
	violatedPkg string // which internal sub-package was illegally imported
}

// checkLayer walks all .go files (excluding _test.go) in layerDir and its
// subdirectories, parses them, and checks every internal import against the
// layer's allowed list.
func checkLayer(t *testing.T, internalDir, layerDir string, rule layerRule) []violation {
	t.Helper()

	var violations []violation
	fset := token.NewFileSet()

	err := filepath.Walk(layerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("failed to parse %s: %v", path, parseErr)
		}

		relPath, _ := filepath.Rel(internalDir, path)

		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			// Only check imports within our module's internal packages.
			if !strings.HasPrefix(importPath, moduleBase+"/") {
				continue
			}

			// Extract the top-level sub-package after internal/.
			// e.g. ".../internal/domain"        -> "domain"
			// e.g. ".../internal/shared/config"  -> "shared"
			suffix := strings.TrimPrefix(importPath, moduleBase+"/")
			subPkg := strings.SplitN(suffix, "/", 2)[0]

			if !isAllowed(subPkg, rule.allowed) {
				violations = append(violations, violation{
					file:        relPath,
					importPath:  importPath,
					violatedPkg: subPkg,
				})
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk %s: %v", layerDir, err)
	}

	return violations
}

// isAllowed returns true if subPkg matches one of the allowed prefixes.
func isAllowed(subPkg string, allowed []string) bool {
	for _, a := range allowed {
		if subPkg == a {
			return true
		}
	}
	return false
}

// findInternalDir locates the internal/ directory by walking up from the
// current working directory (which Go test sets to the package directory).
func findInternalDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Walk up until we find a directory that contains "internal" as a child.
	for {
		candidate := filepath.Join(dir, "internal")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find internal/ directory by walking up from working directory")
		}
		dir = parent
	}
}
