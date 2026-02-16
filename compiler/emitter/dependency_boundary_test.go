package emitter

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSharedEmitter_DoesNotImportNormalizer(t *testing.T) {
	t.Parallel()
	root := repoRootForBoundaryTest(t)
	sharedDir := filepath.Join(root, "compiler", "emitter", "shared")
	checkNoNormalizerImports(t, sharedDir, nil)
}

func TestGoEmitter_OnlyInfraBridgeImportsNormalizer(t *testing.T) {
	t.Parallel()
	root := repoRootForBoundaryTest(t)
	goDir := filepath.Join(root, "compiler", "emitter", "go")
	checkNoNormalizerImports(t, goDir, nil)
}

func checkNoNormalizerImports(t *testing.T, dir string, allow map[string]bool) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}

	fset := token.NewFileSet()
	for _, path := range matches {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		if allow != nil && allow[base] {
			continue
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range file.Imports {
			p := strings.Trim(imp.Path.Value, "\"")
			if p == "github.com/strogmv/ang/compiler/normalizer" {
				t.Fatalf("%s must not import normalizer directly", path)
			}
		}
	}
}

func repoRootForBoundaryTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// compiler/emitter/dependency_boundary_test.go -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
