package compiler

import (
	"strings"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// AttachTemplates copies parsed universal template catalog from CUE infra into IR.
func AttachTemplates(schema *ir.Schema, templates []normalizer.TemplateDef) {
	if schema == nil || len(templates) == 0 {
		return
	}
	out := make([]ir.Template, 0, len(templates))
	for _, t := range templates {
		id := strings.TrimSpace(t.ID)
		if id == "" {
			continue
		}
		out = append(out, ir.Template{
			ID:           id,
			Kind:         strings.TrimSpace(t.Kind),
			Channel:      strings.TrimSpace(t.Channel),
			Locale:       strings.TrimSpace(t.Locale),
			Version:      strings.TrimSpace(t.Version),
			Engine:       strings.TrimSpace(t.Engine),
			Subject:      t.Subject,
			Text:         t.Text,
			HTML:         t.HTML,
			Body:         t.Body,
			RequiredVars: append([]string(nil), t.RequiredVars...),
			OptionalVars: append([]string(nil), t.OptionalVars...),
		})
	}
	if len(out) == 0 {
		return
	}
	schema.Templates = out
}
