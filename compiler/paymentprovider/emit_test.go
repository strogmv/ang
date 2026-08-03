package paymentprovider

import (
	"os"
	"path/filepath"
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
}
