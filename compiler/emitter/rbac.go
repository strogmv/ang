package emitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// rbacReference matches Go code (or a flow action) naming the generated
// package: rbac.CheckPermission, rbac.Roles, rbac.Permissions.
var rbacReference = regexp.MustCompile(`\brbac\.[A-Z]`)

// RBACUsed reports whether the project needs internal/pkg/rbac: it declares
// roles or permissions, an endpoint requires a permission (auth.action), or
// an operation's flow or Go names the package. Otherwise the package would be
// an empty role table nothing reads, and it is not generated.
func RBACUsed(rbac *normalizer.RBACDef, schema *ir.Schema) bool {
	if rbac != nil && (len(rbac.Roles) > 0 || len(rbac.Permissions) > 0) {
		return true
	}
	if schema == nil {
		return false
	}
	for _, ep := range schema.Endpoints {
		if ep.Auth != nil && ep.Auth.Permission != "" {
			return true
		}
	}
	for _, svc := range schema.Services {
		raw, err := json.Marshal(svc.Methods)
		if err != nil {
			return true
		}
		if bytes.Contains(raw, []byte(`"rbac.CheckPermission"`)) || rbacReference.Match(raw) {
			return true
		}
	}
	return false
}

// EmitRBAC generates the RBAC logic based on roles and permissions, when the
// project uses it (RBACUsed).
func (e *Emitter) EmitRBAC(rbac *normalizer.RBACDef) error {
	if !RBACUsed(rbac, e.IRSchema) {
		return nil
	}
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

	targetDir := e.outDir("internal", "pkg", "rbac")
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
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	logGenerated("Generated RBAC: %s\n", path)
	return nil
}
