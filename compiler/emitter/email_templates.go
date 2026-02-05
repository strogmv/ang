package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/strogmv/ang/compiler/normalizer"
)

func (e *Emitter) EmitEmailTemplates(templates []normalizer.EmailTemplateDef) error {
	if len(templates) == 0 {
		return nil
	}
	tmplPath := "templates/email_templates.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	funcMap := template.FuncMap{
		"Quote": func(s string) string {
			return strconv.Quote(s)
		},
	}
	t, err := template.New("email_templates").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "emailtemplates")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, templates); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Printf("Formatting failed for email templates. Writing raw.\n")
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "templates.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}
