package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func resolveEmailTemplates(projectPath string, defs []normalizer.EmailTemplateDef) ([]normalizer.EmailTemplateDef, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	out := make([]normalizer.EmailTemplateDef, 0, len(defs))
	for _, def := range defs {
		resolved := def
		if strings.TrimSpace(resolved.Text) == "" && strings.TrimSpace(resolved.TextFile) != "" {
			textPath := resolved.TextFile
			if !filepath.IsAbs(textPath) {
				textPath = filepath.Join(projectPath, textPath)
			}
			b, err := os.ReadFile(textPath)
			if err != nil {
				return nil, fmt.Errorf("read #EmailTemplates[%s].textFile %q: %w", resolved.Name, resolved.TextFile, err)
			}
			resolved.Text = string(b)
		}
		if strings.TrimSpace(resolved.HTML) == "" && strings.TrimSpace(resolved.HTMLFile) != "" {
			htmlPath := resolved.HTMLFile
			if !filepath.IsAbs(htmlPath) {
				htmlPath = filepath.Join(projectPath, htmlPath)
			}
			b, err := os.ReadFile(htmlPath)
			if err != nil {
				return nil, fmt.Errorf("read #EmailTemplates[%s].htmlFile %q: %w", resolved.Name, resolved.HTMLFile, err)
			}
			resolved.HTML = string(b)
		}
		if strings.TrimSpace(resolved.Text) == "" && strings.TrimSpace(resolved.HTML) == "" {
			return nil, fmt.Errorf("email template %q has neither text nor html content after file resolution", resolved.Name)
		}
		out = append(out, resolved)
	}
	return out, nil
}
