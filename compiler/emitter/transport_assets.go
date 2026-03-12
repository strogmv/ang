package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"

	"github.com/strogmv/ang-ir/normalizer"
)

// emitWSCommon generates the shared WebSocket infrastructure (types, hub, etc.)
func (e *Emitter) emitWSCommon() error {
	tmplPath := "templates/websocket_common.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read ws common template: %w", err)
	}

	funcMap := template.FuncMap{
		"ANGVersion":   func() string { return e.Version },
		"InputHash":    func() string { return e.InputHash },
		"CompilerHash": func() string { return e.CompilerHash },
	}

	t, err := template.New("ws_common").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse ws common template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute ws common template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Printf("Formatting failed for ws_common.go. Writing raw.\n")
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "ws_common.go")
	if err := WriteFileIfChanged(path, formatted, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated WebSocket Common: %s\n", path)
	return nil
}

// EmitHTTPCommon generates shared HTTP middleware.
func (e *Emitter) EmitHTTPCommon(auth *normalizer.AuthDef) error {
	tmplPath := "templates/http_common.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("http_common").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	var buf bytes.Buffer
	if auth == nil {
		auth = &normalizer.AuthDef{
			Alg:              "RS256",
			Issuer:           "",
			Audience:         "",
			UserIDClaim:      "sub",
			CompanyIDClaim:   "cid",
			RolesClaim:       "roles",
			PermissionsClaim: "perms",
		}
	}
	if err := t.Execute(&buf, auth); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "common.go")
	if err := WriteFileIfChanged(path, formatted, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated HTTP Common: %s\n", path)
	return nil
}

// EmitMetrics generates Prometheus middleware.
func (e *Emitter) EmitMetrics() error {
	tmplPath := "templates/metrics.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("metrics").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
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

	path := filepath.Join(targetDir, "metrics.go")
	if err := WriteFileIfChanged(path, formatted, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Metrics Middleware: %s\n", path)
	return nil
}

// EmitLoggingMiddleware generates middleware for structured logging.
func (e *Emitter) EmitLoggingMiddleware() error {
	tmplPath := "templates/logging_middleware.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("logging_middleware").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "logging.go")
	if err := WriteFileIfChanged(path, formatted, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Logging Middleware: %s\n", path)
	return nil
}
