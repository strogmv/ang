package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

func (n *Normalizer) ExtractEmailTemplates(val cue.Value) ([]EmailTemplateDef, error) {
	tmplVal := val.LookupPath(cue.ParsePath("#EmailTemplates"))
	if !tmplVal.Exists() {
		return nil, nil
	}
	iter, err := tmplVal.List()
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
		if name == "" {
			return nil, fmt.Errorf("email template name is required")
		}
		if subject == "" {
			return nil, fmt.Errorf("email template subject is required: %s", name)
		}
		if text == "" && html == "" {
			return nil, fmt.Errorf("email template text or html is required: %s", name)
		}
		templates = append(templates, EmailTemplateDef{
			Name:    name,
			Subject: subject,
			Text:    text,
			HTML:    html,
		})
	}
	return templates, nil
}
