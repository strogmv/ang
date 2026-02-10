package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed init_templates/**
var initTemplatesFS embed.FS

type initTemplateOptions struct {
	TemplateName string
	TargetDir    string
	ProjectName  string
	Lang         string
	DB           string
	ModulePath   string
	Force        bool
}

func initFromTemplate(opts initTemplateOptions) error {
	if opts.TemplateName != "saas" && opts.TemplateName != "ecommerce" && opts.TemplateName != "marketplace" {
		return fmt.Errorf("unknown template %q (allowed: saas, ecommerce, marketplace)", opts.TemplateName)
	}
	if opts.Lang == "" {
		opts.Lang = "go"
	}
	if opts.DB == "" {
		opts.DB = "postgres"
	}

	if err := ensureTargetDir(opts.TargetDir, opts.Force); err != nil {
		return err
	}

	vars := map[string]string{
		"PROJECT_NAME": sanitizeProjectName(opts.ProjectName),
		"MODULE_PATH":  opts.ModulePath,
		"LANG":         opts.Lang,
		"FRAMEWORK":    defaultFrameworkForLang(opts.Lang),
		"DB":           opts.DB,
		"TEMPLATE":     opts.TemplateName,
	}

	if err := writeEmbeddedTree("init_templates/common", opts.TargetDir, vars); err != nil {
		return fmt.Errorf("write common template files: %w", err)
	}
	templateRoot := filepath.ToSlash(filepath.Join("init_templates", "templates", opts.TemplateName))
	if err := writeEmbeddedTree(templateRoot, opts.TargetDir, vars); err != nil {
		return fmt.Errorf("write %s template files: %w", opts.TemplateName, err)
	}

	return nil
}

func ensureTargetDir(dir string, force bool) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create target directory %s: %w", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read target directory %s: %w", dir, err)
	}
	if len(entries) > 0 && !force {
		return fmt.Errorf("target directory %s is not empty (use --force to continue)", dir)
	}
	return nil
}

func writeEmbeddedTree(root string, targetDir string, vars map[string]string) error {
	var files []string
	err := fs.WalkDir(initTemplatesFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, src := range files {
		rel := strings.TrimPrefix(src, root+"/")
		outPath := filepath.Join(targetDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
		}
		data, err := initTemplatesFS.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read embedded file %s: %w", src, err)
		}
		content := applyTemplateVars(string(data), vars)
		if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write file %s: %w", outPath, err)
		}
	}
	return nil
}

func applyTemplateVars(content string, vars map[string]string) string {
	out := content
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func sanitizeProjectName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "ang-app"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "ang-app"
	}
	return out
}

func defaultFrameworkForLang(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "python":
		return "fastapi"
	default:
		return "chi"
	}
}
