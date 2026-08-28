package paymentprovider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A template set decides its own type-file layout: optional templates it ships
// are emitted, the ones it omits are skipped instead of failing generation.
func TestEmitOptionalTypeFiles(t *testing.T) {
	tmplDir := t.TempDir()
	for _, name := range []string{
		"creds.go.tmpl",
		"provider.go.tmpl",
		"provider_test.go.tmpl",
		// model.go.tmpl is provided, datatypes.go.tmpl deliberately is not.
		"model.go.tmpl",
	} {
		if err := os.WriteFile(filepath.Join(tmplDir, name), []byte("package demo\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	outDir := t.TempDir()
	files, err := EmitWithResult(tmplDir, outDir, &TemplateData{
		PackageName:      "demo",
		SigningAlgorithm: "none",
	})
	if err != nil {
		t.Fatalf("EmitWithResult: %v", err)
	}

	emitted := make(map[string]bool, len(files))
	for _, f := range files {
		emitted[f.RelativePath] = true
	}

	for _, want := range []string{"model.go", "creds.go", "demo.go", "demo_test.go"} {
		if !emitted[want] {
			t.Errorf("expected %s to be emitted, got %v", want, files)
		}
	}
	if emitted["datatypes.go"] {
		t.Errorf("datatypes.go emitted although its template is absent, got %v", files)
	}
	if emitted["forms.go"] {
		t.Errorf("forms.go emitted although its template is absent, got %v", files)
	}
}

// A set that hosts its own pages gets them emitted alongside the usual files.
func TestEmitHostedFormsFile(t *testing.T) {
	tmplDir := t.TempDir()
	for _, name := range []string{
		"creds.go.tmpl",
		"provider.go.tmpl",
		"provider_test.go.tmpl",
		"model.go.tmpl",
		"forms.go.tmpl",
	} {
		if err := os.WriteFile(filepath.Join(tmplDir, name), []byte("package demo\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	files, err := EmitWithResult(tmplDir, t.TempDir(), &TemplateData{
		PackageName:      "demo",
		SigningAlgorithm: "none",
	})
	if err != nil {
		t.Fatalf("EmitWithResult: %v", err)
	}

	for _, f := range files {
		if f.RelativePath == "forms.go" {
			return
		}
	}
	t.Errorf("expected forms.go to be emitted, got %v", files)
}

// Template sets share blocks through module_dirs instead of copying a library
// into every set; a set that redefines a shared block keeps its own version.
func TestEmitSharedModuleDirs(t *testing.T) {
	shared := t.TempDir()
	if err := os.MkdirAll(filepath.Join(shared, "settlement"), 0o755); err != nil {
		t.Fatalf("mkdir shared group: %v", err)
	}
	writeFile(t, filepath.Join(shared, "settlement", "requisites.go.tmpl"),
		`{{ define "settlement" }}// shared settlement{{ end }}`)
	writeFile(t, filepath.Join(shared, "logging.go.tmpl"),
		`{{ define "logging" }}// shared logging{{ end }}`)

	tmplDir := t.TempDir()
	writeFile(t, filepath.Join(tmplDir, "creds.go.tmpl"), "package demo\n")
	writeFile(t, filepath.Join(tmplDir, "provider_test.go.tmpl"), "package demo\n")
	writeFile(t, filepath.Join(tmplDir, "provider.go.tmpl"),
		"package demo\n\n{{ template \"settlement\" . }}\n{{ template \"logging\" . }}\n")
	// The set keeps its own copy of one block only.
	if err := os.MkdirAll(filepath.Join(tmplDir, "modules"), 0o755); err != nil {
		t.Fatalf("mkdir set modules: %v", err)
	}
	writeFile(t, filepath.Join(tmplDir, "modules", "logging.go.tmpl"),
		`{{ define "logging" }}// set logging{{ end }}`)

	outDir := t.TempDir()
	if _, err := EmitWithResult(tmplDir, outDir, &TemplateData{
		PackageName:      "demo",
		SigningAlgorithm: "none",
	}, shared); err != nil {
		t.Fatalf("EmitWithResult: %v", err)
	}

	generated, err := os.ReadFile(filepath.Join(outDir, "demo.go"))
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}
	got := string(generated)
	if !strings.Contains(got, "// shared settlement") {
		t.Errorf("shared block was not visible to the set:\n%s", got)
	}
	if !strings.Contains(got, "// set logging") || strings.Contains(got, "// shared logging") {
		t.Errorf("set-local block must win over the shared one:\n%s", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
