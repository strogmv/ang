package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func resolveTemplates(projectPath string, defs []normalizer.TemplateDef) ([]normalizer.TemplateDef, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	out := make([]normalizer.TemplateDef, 0, len(defs))
	for _, def := range defs {
		resolved := def

		if strings.TrimSpace(resolved.Subject) == "" && strings.TrimSpace(resolved.SubjectFile) != "" {
			v, err := readTemplateFile(projectPath, resolved.SubjectFile)
			if err != nil {
				return nil, fmt.Errorf("read #Templates[%s].subjectFile: %w", resolved.ID, err)
			}
			resolved.Subject = v
		}
		if strings.TrimSpace(resolved.Text) == "" && strings.TrimSpace(resolved.TextFile) != "" {
			v, err := readTemplateFile(projectPath, resolved.TextFile)
			if err != nil {
				return nil, fmt.Errorf("read #Templates[%s].textFile: %w", resolved.ID, err)
			}
			resolved.Text = v
		}
		if strings.TrimSpace(resolved.HTML) == "" && strings.TrimSpace(resolved.HTMLFile) != "" {
			v, err := readTemplateFile(projectPath, resolved.HTMLFile)
			if err != nil {
				return nil, fmt.Errorf("read #Templates[%s].htmlFile: %w", resolved.ID, err)
			}
			resolved.HTML = v
		}
		if strings.TrimSpace(resolved.Body) == "" && strings.TrimSpace(resolved.BodyFile) != "" {
			v, err := readTemplateFile(projectPath, resolved.BodyFile)
			if err != nil {
				return nil, fmt.Errorf("read #Templates[%s].bodyFile: %w", resolved.ID, err)
			}
			resolved.Body = v
		}
		out = append(out, resolved)
	}
	return out, nil
}

func templatesToEmail(defs []normalizer.TemplateDef) ([]normalizer.EmailTemplateDef, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	out := make([]normalizer.EmailTemplateDef, 0, len(defs))
	for _, t := range defs {
		if !strings.EqualFold(t.Channel, "email") && !strings.EqualFold(t.Kind, "email") {
			continue
		}
		if strings.TrimSpace(t.Subject) == "" {
			return nil, fmt.Errorf("email template %q requires subject", t.ID)
		}
		text := t.Text
		if strings.TrimSpace(text) == "" {
			text = t.Body
		}
		html := t.HTML
		if strings.TrimSpace(html) == "" && strings.Contains(strings.ToLower(t.Channel), "html") {
			html = t.Body
		}
		if strings.TrimSpace(text) == "" && strings.TrimSpace(html) == "" {
			return nil, fmt.Errorf("email template %q requires text/html/body content", t.ID)
		}
		out = append(out, normalizer.EmailTemplateDef{
			Name:    t.ID,
			Subject: t.Subject,
			Text:    text,
			HTML:    html,
		})
	}
	return out, nil
}

func readTemplateFile(projectPath, path string) (string, error) {
	filePath := path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(projectPath, filePath)
	}
	b, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
