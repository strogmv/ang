package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/strogmv/ang/compiler/normalizer"
)

// EmitTestStubs generates vitest stubs for provided endpoints into a specific file.
func (e *Emitter) EmitTestStubs(endpoints []normalizer.Endpoint, filename string) error {
	tmplPath := filepath.Join(e.TemplatesDir, "test_stub.tmpl")
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read test stub template: %w", err)
	}

	funcMap := e.getSharedFuncMap()

	t, err := template.New("test_stubs").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse test stub template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "tests", "generated")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir tests/generated: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, struct {
		Endpoints []normalizer.Endpoint
	}{
		Endpoints: endpoints,
	}); err != nil {
		return fmt.Errorf("execute test stub template: %w", err)
	}

	if filename == "" {
		filename = "endpoint-stubs.test.ts"
	}
	path := filepath.Join(targetDir, filename)
	if err := WriteFileIfChanged(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write test stubs file: %w", err)
	}

	fmt.Printf("Generated Test Stubs: %s\n", path)
	return nil
}
