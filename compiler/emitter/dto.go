package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
)

// DTOTemplateData wraps entity data for DTO template.
type DTOTemplateData struct {
	ir.Entity
	Imports    []string
	ANGVersion string
}

// EmitDTO generates DTO files using IR.
func (e *Emitter) EmitDTO(entities []ir.Entity) error {
	tmplPath := "templates/dto.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", tmplPath, err)
	}

	funcMap := e.getSharedFuncMap()
	funcMap["GoType"] = IRTypeRefToGoType
	funcMap["TrimDomain"] = func(s string) string { return strings.ReplaceAll(s, "domain.", "") }
	funcMap["DTOType"] = IRTypeRefToDTOType

	t, err := template.New("dto").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "dto")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	for _, entity := range entities {
		// Only generate DTO if there are fields or it's a domain entity
		if len(entity.Fields) == 0 {
			continue
		}

		data := DTOTemplateData{
			Entity:     entity,
			Imports:    computeDTOImports(entity),
			ANGVersion: e.Version,
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to execute template for %s: %w", entity.Name, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			formatted = buf.Bytes()
		}

		filename := strings.ToLower(entity.Name) + ".go"
		path := filepath.Join(targetDir, filename)

		if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", path, err)
		}
		fmt.Printf("Generated DTO: %s\n", path)
	}

	return nil
}

// IRTypeRefToDTOType converts IR TypeRef to DTO type string.
// For entity types, it adds "DTO" suffix since we're in the dto package.
func IRTypeRefToDTOType(t ir.TypeRef) string {
	switch t.Kind {
	case ir.KindString:
		return "string"
	case ir.KindInt:
		return "int"
	case ir.KindInt64:
		return "int64"
	case ir.KindFloat:
		return "float64"
	case ir.KindBool:
		return "bool"
	case ir.KindTime:
		return "time.Time"
	case ir.KindUUID:
		return "string"
	case ir.KindJSON:
		return "json.RawMessage"
	case ir.KindAny:
		return "any"
	case ir.KindList:
		if t.ItemType != nil {
			return "[]" + IRTypeRefToDTOType(*t.ItemType)
		}
		return "[]any"
	case ir.KindMap:
		keyType := "string"
		valType := "any"
		if t.KeyType != nil {
			keyType = IRTypeRefToDTOType(*t.KeyType)
		}
		if t.ItemType != nil {
			valType = IRTypeRefToDTOType(*t.ItemType)
		}
		return "map[" + keyType + "]" + valType
	case ir.KindEntity:
		if t.Name != "" {
			return t.Name + "DTO"
		}
		return "any"
	case ir.KindEnum:
		return "string"
	case ir.KindFile:
		return "string"
	default:
		return "any"
	}
}
