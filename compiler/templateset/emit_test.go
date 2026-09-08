package templateset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_schemaDirAndOutputs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ang.yaml"), []byte(`
cue_root: ".cue"
templates_dir: "../.ang/templates"
schema_dir: "../.ang/schema"
module_dirs:
  - "../.ang/lib"
outputs:
  provider.go.tmpl: "{package}.go"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pc := LoadConfig(dir)
	if pc.SchemaDir != "../.ang/schema" {
		t.Fatalf("schema_dir: %q", pc.SchemaDir)
	}
	if pc.Outputs["provider.go.tmpl"] != "{package}.go" {
		t.Fatalf("outputs: %#v", pc.Outputs)
	}
	if len(pc.ModuleDirs) != 1 || pc.ModuleDirs[0] != "../.ang/lib" {
		t.Fatalf("module_dirs: %#v", pc.ModuleDirs)
	}
	resolved, err := ResolvePath(dir, pc.SchemaDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(resolved)) != ".ang" {
		t.Fatalf("resolved: %s", resolved)
	}
}

func TestEmit_declaredOutputsAndSharedModules(t *testing.T) {
	shared := t.TempDir()
	if err := os.MkdirAll(filepath.Join(shared, "settlement"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(shared, "settlement", "pay.go.tmpl"),
		`{{ define "settlement" }}// shared settlement{{ end }}`)
	writeFile(t, filepath.Join(shared, "logging.go.tmpl"),
		`{{ define "logging" }}// shared logging{{ end }}`)

	tmplDir := t.TempDir()
	writeFile(t, filepath.Join(tmplDir, "provider.go.tmpl"),
		"package demo\n\n{{ template \"settlement\" . }}\n{{ template \"logging\" . }}\n")
	if err := os.MkdirAll(filepath.Join(tmplDir, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(tmplDir, "modules", "logging.go.tmpl"),
		`{{ define "logging" }}// set logging{{ end }}`)

	outDir := t.TempDir()
	files, err := Emit(tmplDir, outDir, struct{ PackageName string }{PackageName: "demo"}, Options{
		ModuleDirs: []string{shared},
		Outputs:    map[string]string{"provider.go.tmpl": "{package}.go"},
	})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(files) != 1 || files[0].RelativePath != "demo.go" {
		t.Fatalf("files: %#v", files)
	}

	generated, err := os.ReadFile(filepath.Join(outDir, "demo.go"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(generated)
	if !strings.Contains(got, "// shared settlement") {
		t.Errorf("shared block missing:\n%s", got)
	}
	if !strings.Contains(got, "// set logging") || strings.Contains(got, "// shared logging") {
		t.Errorf("set-local block must win:\n%s", got)
	}
}

func TestEmit_nilData(t *testing.T) {
	_, err := Emit(t.TempDir(), t.TempDir(), nil, Options{Outputs: map[string]string{"a.go.tmpl": "a.go"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
