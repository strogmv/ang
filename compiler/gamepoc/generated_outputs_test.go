package gamepoc

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedManifestIncludesIgnoredOutputs(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q", root}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("presentation.json\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(root, "tracked.go")
	ignored := filepath.Join(root, "presentation.json")
	for _, path := range []string{tracked, ignored} {
		if err := os.WriteFile(path, []byte("same bytes\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec.Command("git", "-C", root, "add", "tracked.go").Run(); err != nil {
		t.Fatal(err)
	}
	if err := writeGeneratedOutputs(root, []string{ignored, tracked, ignored}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".local/ang/generated-outputs.json")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files []generatedOutput `json:"files"`
	}
	if err := json.Unmarshal(first, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 2 || manifest.Files[0].Path != "presentation.json" || manifest.Files[0].Tracked || !manifest.Files[1].Tracked {
		t.Fatalf("inventory: %+v", manifest.Files)
	}
	if err := writeGeneratedOutputs(root, []string{tracked, ignored}); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil || string(first) != string(second) {
		t.Fatal("manifest is not deterministic", err)
	}
	if err := os.Remove(ignored); err != nil {
		t.Fatal(err)
	}
	if err := writeGeneratedOutputs(root, []string{ignored}); err == nil {
		t.Fatal("missing generated output accepted")
	}
	if err := writeGeneratedOutputs(root, []string{filepath.Join(root, "../escape")}); err == nil {
		t.Fatal("escaping generated output accepted")
	}
}

// An emitter must not silently bypass the inventory when it adds a new output.
func TestGenerationFileWritesGoThroughInventory(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "generated_outputs.go" || name == "wounds_memo.go" {
			continue // the shared writer and the explicitly local diagnostic cache
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "WriteFile" {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if ok && pkg.Name == "os" {
				t.Errorf("%s: generated file write bypasses inventory; use writeGeneratedFile", set.Position(call.Pos()))
			}
			return true
		})
	}
}

func TestGeneratedInventoryRecordsOnlySuccessfulWrites(t *testing.T) {
	root := t.TempDir()
	end := beginGeneratedOutputs()
	defer end()
	if err := writeGeneratedFile(filepath.Join(root, "missing", "not-written.json"), []byte("{}"), 0644); err == nil {
		t.Fatal("write should fail")
	}
	path := filepath.Join(root, "presentation.json")
	if err := writeGeneratedFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := finishGeneratedOutputs(filepath.Join(root, "server")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".local/ang/generated-outputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files []generatedOutput `json:"files"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "presentation.json" {
		t.Fatalf("wrong recorded writes: %+v", manifest.Files)
	}
}
