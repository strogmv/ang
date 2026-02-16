package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"
)

// EmitMailerPort генерирует интерфейс для отправки почты
func (e *Emitter) EmitMailerPort() error {
	tmplPath := filepath.Join(e.TemplatesDir, "mailer_port.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/mailer_port.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("mailer_port").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "mailer.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Mailer Port: %s\n", path)
	return nil
}

// EmitMailerAdapter генерирует реализацию Mailer через SMTP
func (e *Emitter) EmitMailerAdapter() error {
	tmplPath := filepath.Join(e.TemplatesDir, "smtp_client.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/smtp_client.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("smtp_client").Funcs(template.FuncMap{
		"GoModule": func() string { return e.GoModule },
	}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "mailer", "smtp")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "client.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated SMTP Mailer Adapter: %s\n", path)
	return nil
}
