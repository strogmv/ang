package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// computeDomainImports determines required imports for domain entities.
func computeDomainImports(ent ir.Entity) []string {
	imports := make(map[string]bool)
	hasFileFields := false
	hasConstraints := false
	for _, f := range ent.Fields {
		if f.Type.Kind == ir.KindTime {
			imports["time"] = true
		}
		if f.Constraints != nil {
			hasConstraints = true
		}
		// Check for file attributes
		for _, attr := range f.Attributes {
			if attr.Name == "file" || attr.Name == "image" {
				hasFileFields = true
			}
		}
	}

	// Add log/slog for LogValuer interface (only if fields exist)
	if len(ent.Fields) > 0 {
		imports["log/slog"] = true
	}

	// Add fmt if needed for FSM error messages or file previews
	if ent.FSM != nil || hasFileFields || hasConstraints {
		imports["fmt"] = true
	}

	result := make([]string, 0, len(imports))
	for imp := range imports {
		result = append(result, imp)
	}
	sort.Strings(result)
	return result
}

// computeDTOImports determines required imports for DTOs.
func computeDTOImports(ent ir.Entity) []string {
	imports := make(map[string]bool)
	for _, f := range ent.Fields {
		if f.Type.Kind == ir.KindTime {
			imports["time"] = true
		}
		if f.Constraints != nil {
			imports["fmt"] = true
		}
	}

	result := make([]string, 0, len(imports))
	for imp := range imports {
		result = append(result, imp)
	}
	sort.Strings(result)
	return result
}

// DomainTemplateData wraps entity data for domain template.
type DomainTemplateData struct {
	ir.Entity
	Imports      []string
	ANGVersion   string
	InputHash    string
	CompilerHash string
}

// EmitDomain generates domain entity files using IR.
func (e *Emitter) EmitDomain(entities []ir.Entity) error {
	tmplPath := "templates/domain.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", tmplPath, err)
	}

	funcMap := e.getSharedFuncMap()
	funcMap["GoType"] = IRTypeRefToGoType
	funcMap["TrimDomain"] = func(s string) string { return strings.ReplaceAll(s, "domain.", "") }
	funcMap["HasTime"] = func(fields []ir.Field) bool {
		for _, f := range fields {
			if f.Type.Kind == ir.KindTime {
				return true
			}
		}
		return false
	}
	funcMap["HasFileFields"] = func(fields []ir.Field) bool {
		for _, f := range fields {
			for _, attr := range f.Attributes {
				if attr.Name == "file" || attr.Name == "image" {
					return true
				}
			}
		}
		return false
	}

	t, err := template.New("domain").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "domain")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	for _, entity := range entities {
		// Compute imports based on IR kinds
		imports := computeDomainImports(entity)

		data := DomainTemplateData{
			Entity:       entity,
			Imports:      imports,
			ANGVersion:   e.Version,
			InputHash:    e.InputHash,
			CompilerHash: e.CompilerHash,
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to execute template for %s: %w", entity.Name, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to format source for %s: %w", entity.Name, err)
		}

		filename := strings.ToLower(entity.Name) + ".go"
		path := filepath.Join(targetDir, filename)

		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", path, err)
		}
		fmt.Printf("Generated: %s\n", path)
	}

	return nil
}

// EmitEvents generates event structs.
func (e *Emitter) EmitEvents(events []normalizer.EventDef) error {
	tmplPath := "templates/event_structs.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("event_structs").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "domain")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, events); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "events.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Events: %s\n", path)
	return nil
}
