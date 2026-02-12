package normalizer

import (
	"fmt"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
)

func (n *Normalizer) ExtractEmailTemplates(val cue.Value) ([]EmailTemplateDef, error) {
	tmplVal := val.LookupPath(cue.ParsePath("#EmailTemplates"))
	if !tmplVal.Exists() {
		return nil, nil
	}

	baseDir := ""
	itemsVal := tmplVal
	if tmplVal.Kind() == cue.StructKind {
		baseDir = strings.TrimSpace(getString(tmplVal, "dir"))
		itemsVal = tmplVal.LookupPath(cue.ParsePath("items"))
		if !itemsVal.Exists() {
			return nil, fmt.Errorf("#EmailTemplates.items is required when #EmailTemplates is a struct")
		}
	}

	iter, err := itemsVal.List()
	if err != nil {
		return nil, fmt.Errorf("email templates must be a list: %w", err)
	}
	var templates []EmailTemplateDef
	for iter.Next() {
		v := iter.Value()
		name := strings.TrimSpace(getString(v, "name"))
		subject := getString(v, "subject")
		text := getString(v, "text")
		html := getString(v, "html")
		textFile := strings.TrimSpace(getString(v, "textFile"))
		htmlFile := strings.TrimSpace(getString(v, "htmlFile"))
		if name == "" {
			return nil, fmt.Errorf("email template name is required")
		}
		if subject == "" {
			return nil, fmt.Errorf("email template subject is required: %s", name)
		}
		if baseDir != "" {
			if textFile != "" && !filepath.IsAbs(textFile) {
				textFile = filepath.Join(baseDir, textFile)
			}
			if htmlFile != "" && !filepath.IsAbs(htmlFile) {
				htmlFile = filepath.Join(baseDir, htmlFile)
			}
		}
		if text == "" && html == "" && textFile == "" && htmlFile == "" {
			return nil, fmt.Errorf("email template text/html or textFile/htmlFile is required: %s", name)
		}
		templates = append(templates, EmailTemplateDef{
			Name:     name,
			Subject:  subject,
			Text:     text,
			HTML:     html,
			TextFile: textFile,
			HTMLFile: htmlFile,
		})
	}
	return templates, nil
}

// ExtractTemplates parses universal template catalog from #Templates.
// Supports either:
//
//	#Templates: [{...}]
//
// or:
//
//	#Templates: { dir: "...", items: [{...}] }
func (n *Normalizer) ExtractTemplates(val cue.Value) ([]TemplateDef, error) {
	tmplVal := val.LookupPath(cue.ParsePath("#Templates"))
	if !tmplVal.Exists() {
		return nil, nil
	}

	baseDir := ""
	itemsVal := tmplVal
	if tmplVal.Kind() == cue.StructKind {
		baseDir = strings.TrimSpace(getString(tmplVal, "dir"))
		itemsVal = tmplVal.LookupPath(cue.ParsePath("items"))
		if !itemsVal.Exists() {
			return nil, fmt.Errorf("#Templates.items is required when #Templates is a struct")
		}
	}

	iter, err := itemsVal.List()
	if err != nil {
		return nil, fmt.Errorf("templates must be a list: %w", err)
	}
	var templates []TemplateDef
	for iter.Next() {
		v := iter.Value()
		id := strings.TrimSpace(getString(v, "id"))
		if id == "" {
			id = strings.TrimSpace(getString(v, "name"))
		}
		if id == "" {
			return nil, fmt.Errorf("template id/name is required")
		}

		kind := strings.TrimSpace(getString(v, "kind"))
		channel := strings.TrimSpace(getString(v, "channel"))
		if kind == "" {
			kind = "generic"
		}
		if channel == "" && strings.EqualFold(kind, "email") {
			channel = "email"
		}

		item := TemplateDef{
			ID:           id,
			Kind:         kind,
			Channel:      channel,
			Locale:       strings.TrimSpace(getString(v, "locale")),
			Version:      strings.TrimSpace(getString(v, "version")),
			Engine:       strings.TrimSpace(getString(v, "engine")),
			Subject:      getString(v, "subject"),
			Text:         getString(v, "text"),
			HTML:         getString(v, "html"),
			Body:         getString(v, "body"),
			SubjectFile:  strings.TrimSpace(getString(v, "subjectFile")),
			TextFile:     strings.TrimSpace(getString(v, "textFile")),
			HTMLFile:     strings.TrimSpace(getString(v, "htmlFile")),
			BodyFile:     strings.TrimSpace(getString(v, "bodyFile")),
			RequiredVars: getStringList(v, "requiredVars"),
			OptionalVars: getStringList(v, "optionalVars"),
		}
		if item.Engine == "" {
			item.Engine = "go_template"
		}
		if len(item.RequiredVars) == 0 {
			item.RequiredVars = getStringList(v, "vars.required")
		}
		if len(item.OptionalVars) == 0 {
			item.OptionalVars = getStringList(v, "vars.optional")
		}
		item.RequiredVars = normalizeStringList(item.RequiredVars)
		item.OptionalVars = normalizeStringList(item.OptionalVars)

		if baseDir != "" {
			if item.SubjectFile != "" && !filepath.IsAbs(item.SubjectFile) {
				item.SubjectFile = filepath.Join(baseDir, item.SubjectFile)
			}
			if item.TextFile != "" && !filepath.IsAbs(item.TextFile) {
				item.TextFile = filepath.Join(baseDir, item.TextFile)
			}
			if item.HTMLFile != "" && !filepath.IsAbs(item.HTMLFile) {
				item.HTMLFile = filepath.Join(baseDir, item.HTMLFile)
			}
			if item.BodyFile != "" && !filepath.IsAbs(item.BodyFile) {
				item.BodyFile = filepath.Join(baseDir, item.BodyFile)
			}
		}

		hasInline := item.Subject != "" || item.Text != "" || item.HTML != "" || item.Body != ""
		hasFile := item.SubjectFile != "" || item.TextFile != "" || item.HTMLFile != "" || item.BodyFile != ""
		if !hasInline && !hasFile {
			return nil, fmt.Errorf("template %q has no content (subject/text/html/body or corresponding *File)", item.ID)
		}
		templates = append(templates, item)
	}
	return templates, nil
}

func getStringList(v cue.Value, path string) []string {
	listVal := v.LookupPath(cue.ParsePath(path))
	if !listVal.Exists() {
		return nil
	}
	iter, err := listVal.List()
	if err != nil {
		return nil
	}
	out := make([]string, 0, 4)
	for iter.Next() {
		s, err := iter.Value().String()
		if err != nil {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeStringList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, v := range in {
		s := strings.TrimSpace(v)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
