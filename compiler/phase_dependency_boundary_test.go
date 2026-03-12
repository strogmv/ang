package compiler

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompilerPhaseBoundaries_NoReverseImports(t *testing.T) {
	t.Parallel()

	root := repoRootForPhaseBoundaryTest(t)
	angIRRoot := angIRRootForPhaseBoundaryTest(t)
	type phaseRule struct {
		name         string
		dir          string
		modulePrefix string
		forbid       []string
	}

	rules := []phaseRule{
		{
			name:         "parser",
			dir:          filepath.Join(angIRRoot, "parser"),
			modulePrefix: "github.com/strogmv/ang-ir",
			forbid:       []string{"normalizer", "flowsem", "ir"},
		},
		{
			name:         "normalizer",
			dir:          filepath.Join(angIRRoot, "normalizer"),
			modulePrefix: "github.com/strogmv/ang-ir",
			forbid:       []string{"flowsem", "ir"},
		},
		{
			name:         "flowsem",
			dir:          filepath.Join(angIRRoot, "flowsem"),
			modulePrefix: "github.com/strogmv/ang-ir",
			forbid:       []string{"ir"},
		},
		{
			name:         "ir",
			dir:          filepath.Join(angIRRoot, "ir"),
			modulePrefix: "github.com/strogmv/ang-ir",
			forbid:       nil,
		},
		{
			name:         "emitter",
			dir:          filepath.Join(root, "compiler", "emitter"),
			modulePrefix: "github.com/strogmv/ang-ir",
			forbid:       []string{"parser", "flowsem"},
		},
	}

	for _, rule := range rules {
		rule := rule
		t.Run(rule.name, func(t *testing.T) {
			t.Parallel()
			assertNoPhaseImports(t, rule.dir, rule.modulePrefix, rule.forbid)
		})
	}
}

func assertNoPhaseImports(t *testing.T, dir, modulePrefix string, forbiddenPhases []string) {
	t.Helper()

	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			for _, forbidden := range forbiddenPhases {
				prefix := modulePrefix + "/" + forbidden
				if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
					t.Fatalf("%s imports forbidden phase dependency %q", path, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
}

func repoRootForPhaseBoundaryTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// compiler/phase_dependency_boundary_test.go -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func angIRRootForPhaseBoundaryTest(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(repoRootForPhaseBoundaryTest(t), "..", "ang-ir"))
}
