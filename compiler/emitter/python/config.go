package python

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

type configField struct {
	Name        string
	Type        string
	DefaultExpr string
}

type configData struct {
	Fields []configField
}

func EmitConfig(outputDir string, rt Runtime, cfg *normalizer.ConfigDef) error {
	fields := make([]configField, 0)
	if cfg != nil {
		for _, f := range cfg.Fields {
			fields = append(fields, configField{
				Name:        ToSnake(f.Name),
				Type:        pythonConfigType(f),
				DefaultExpr: pythonConfigDefault(f),
			})
		}
	}
	data := configData{Fields: fields}
	root := filepath.Join(outputDir, "app")
	return rt.RenderTemplate(root, "templates/python/fastapi/config.py.tmpl", data, "config.py", 0644)
}

func pythonConfigType(f normalizer.Field) string {
	t := strings.ToLower(strings.TrimSpace(f.Type))
	switch t {
	case "int", "int64", "int32", "uint", "uint64", "uint32":
		return "int"
	case "float", "float64", "float32", "number", "decimal":
		return "float"
	case "bool", "boolean":
		return "bool"
	case "json", "map[string]any", "map[string]interface{}", "object":
		return "dict[str, Any]"
	default:
		return "str"
	}
}

func pythonConfigDefault(f normalizer.Field) string {
	if s := strings.TrimSpace(f.Default); s != "" {
		switch pythonConfigType(f) {
		case "int":
			if _, err := strconv.Atoi(s); err == nil {
				return s
			}
		case "float":
			if _, err := strconv.ParseFloat(s, 64); err == nil {
				return s
			}
		case "bool":
			ls := strings.ToLower(s)
			if ls == "true" || ls == "false" {
				return strings.Title(ls)
			}
		}
		return fmt.Sprintf("%q", s)
	}
	if f.IsOptional {
		return "None"
	}
	switch pythonConfigType(f) {
	case "int":
		return "0"
	case "float":
		return "0.0"
	case "bool":
		return "False"
	case "dict[str, Any]":
		return "{}"
	default:
		return `""`
	}
}
