package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"

	"github.com/strogmv/ang/compiler/normalizer"
)

// EmitRBAC generates the RBAC logic based on roles and permissions.
func (e *Emitter) EmitRBAC(rbac *normalizer.RBACDef) error {
	if rbac == nil {
		rbac = &normalizer.RBACDef{
			Roles:       make(map[string][]string),
			Permissions: make(map[string]string),
		}
	}
	tmplPath := filepath.Join(e.TemplatesDir, "rbac.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/rbac.tmpl"
	}
	
tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("rbac").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "rbac")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, rbac); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "rbac.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated RBAC: %s\n", path)
	return nil
}