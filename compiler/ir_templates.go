package compiler

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

// AttachTemplates copies parsed universal template catalog from CUE infra into IR.
// If basePath is provided, *File fields are resolved relative to it.
func AttachTemplates(schema *ir.Schema, templates []normalizer.TemplateDef, basePath string) {
	if schema == nil || len(templates) == 0 {
		return
	}
	readFile := func(path string) string {
		if path == "" {
			return ""
		}
		if basePath != "" && !filepath.IsAbs(path) {
			path = filepath.Join(basePath, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return string(data)
	}
	out := make([]ir.Template, 0, len(templates))
	for _, t := range templates {
		id := strings.TrimSpace(t.ID)
		if id == "" {
			continue
		}
		htmlContent := t.HTML
		if htmlContent == "" {
			htmlContent = readFile(t.HTMLFile)
		}
		textContent := t.Text
		if textContent == "" {
			textContent = readFile(t.TextFile)
		}
		bodyContent := t.Body
		if bodyContent == "" {
			bodyContent = readFile(t.BodyFile)
		}
		subjectContent := t.Subject
		if subjectContent == "" {
			subjectContent = readFile(t.SubjectFile)
		}
		out = append(out, ir.Template{
			ID:           id,
			Kind:         strings.TrimSpace(t.Kind),
			Channel:      strings.TrimSpace(t.Channel),
			Locale:       strings.TrimSpace(t.Locale),
			Version:      strings.TrimSpace(t.Version),
			Engine:       strings.TrimSpace(t.Engine),
			Subject:      subjectContent,
			Text:         textContent,
			HTML:         htmlContent,
			Body:         bodyContent,
			RequiredVars: append([]string(nil), t.RequiredVars...),
			OptionalVars: append([]string(nil), t.OptionalVars...),
		})
	}
	if len(out) == 0 {
		return
	}
	schema.Templates = out
}
