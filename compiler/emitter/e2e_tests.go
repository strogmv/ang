package emitter

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/strogmv/ang/compiler/normalizer"
)

func (e *Emitter) EmitE2ETests(scenarios []normalizer.ScenarioDef) error {
	if len(scenarios) == 0 {
		return nil
	}

	tmplPath := filepath.Join(e.TemplatesDir, "e2e_tests.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/e2e_tests.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("e2e_tests").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "tests", "e2e")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	path := filepath.Join(targetDir, "behavioral_test.go")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	data := struct {
		Scenarios []normalizer.ScenarioDef
	}{
		Scenarios: scenarios,
	}

	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	fmt.Printf("Generated E2E Behavioral Tests: %s\n", path)
	return nil
}
