package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

type emailTemplateRenderModel struct {
	Name         string
	Subject      string
	Text         string
	HTML         string
	RequiredVars []string
	OptionalVars []string
}

func buildEmailTemplateRenderModels(schema *ir.Schema, defs []normalizer.EmailTemplateDef) []emailTemplateRenderModel {
	if len(defs) == 0 && (schema == nil || len(schema.Templates) == 0) {
		return nil
	}
	metaByID := map[string]ir.Template{}
	if schema != nil {
		for _, tpl := range schema.Templates {
			if tpl.ID == "" {
				continue
			}
			if tpl.Channel != "email" && tpl.Kind != "email" {
				continue
			}
			metaByID[tpl.ID] = tpl
		}
	}
	out := make([]emailTemplateRenderModel, 0, len(defs))
	for _, def := range defs {
		if def.Name == "" {
			continue
		}
		item := emailTemplateRenderModel{
			Name:    def.Name,
			Subject: def.Subject,
			Text:    def.Text,
			HTML:    def.HTML,
		}
		if meta, ok := metaByID[def.Name]; ok {
			item.RequiredVars = append([]string(nil), meta.RequiredVars...)
			item.OptionalVars = append([]string(nil), meta.OptionalVars...)
		}
		out = append(out, item)
	}
	return out
}

func (e *Emitter) EmitEmailTemplates(templates []normalizer.EmailTemplateDef) error {
	return e.EmitEmailTemplatesFromIR(nil, templates)
}

func (e *Emitter) EmitEmailTemplatesFromIR(schema *ir.Schema, templates []normalizer.EmailTemplateDef) error {
	models := buildEmailTemplateRenderModels(schema, templates)
	if len(models) == 0 {
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
		"GoModule": func() string {
			return e.GoModule
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
	if err := t.Execute(&buf, models); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Printf("Formatting failed for email templates. Writing raw.\n")
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "templates.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}
