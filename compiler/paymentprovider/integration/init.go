package integration

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// InitOptions configures ang pp init scaffolding.
// Domain content lives in the consumer repo's .ang/init templates, not here.
type InitOptions struct {
	ProjectPath   string
	InitDir       string // consumer .ang/init; empty → walk up from ProjectPath
	SID           string
	Label         string
	Name          string
	PackageName   string
	Module        string
	TicketSummary string
	KnowledgeID   string // Expert knowledge/data/<id>.json (default: sid)
	Force         bool
}

// InitData is the generic substitution context for consumer init templates.
type InitData struct {
	PackageName   string
	SID           string
	Label         string
	Name          string
	Module        string
	KnowledgeID   string
	TicketSummary string
}

// InitResult lists files created by InitProject.
type InitResult struct {
	Created []string
	Skipped []string
}

// InitProject renders *.tmpl from the consumer .ang/init tree into ProjectPath.
func InitProject(opts InitOptions) (InitResult, error) {
	opts = normalizeInitOptions(opts)
	if strings.TrimSpace(opts.SID) == "" {
		return InitResult{}, fmt.Errorf("sid is required")
	}
	if strings.TrimSpace(opts.Label) == "" {
		return InitResult{}, fmt.Errorf("label is required")
	}
	initDir, err := resolveInitDir(opts.ProjectPath, opts.InitDir)
	if err != nil {
		return InitResult{}, err
	}

	data := InitData{
		PackageName:   opts.PackageName,
		SID:           opts.SID,
		Label:         opts.Label,
		Name:          opts.Name,
		Module:        opts.Module,
		KnowledgeID:   opts.KnowledgeID,
		TicketSummary: opts.TicketSummary,
	}
	tmpl := template.New("init").Option("missingkey=error").Funcs(initFuncMap())

	if err := os.MkdirAll(opts.ProjectPath, 0o755); err != nil {
		return InitResult{}, err
	}

	var result InitResult
	rendered := 0
	err = filepath.WalkDir(initDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".tmpl") {
			return nil
		}
		rel, err := filepath.Rel(initDir, path)
		if err != nil {
			return err
		}
		outRel := strings.TrimSuffix(rel, ".tmpl")
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		parsed, err := tmpl.New(rel).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		var buf bytes.Buffer
		if err := parsed.Execute(&buf, data); err != nil {
			return fmt.Errorf("render %s: %w", rel, err)
		}
		rendered++
		return writeInitFile(opts, &result, outRel, buf.Bytes())
	})
	if err != nil {
		return result, err
	}
	if rendered == 0 {
		return result, fmt.Errorf("pp init: no *.tmpl files in %s", initDir)
	}
	return result, nil
}

func writeInitFile(opts InitOptions, result *InitResult, rel string, content []byte) error {
	target := filepath.Join(opts.ProjectPath, rel)
	if _, err := os.Stat(target); err == nil && !opts.Force {
		result.Skipped = append(result.Skipped, rel)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return err
	}
	result.Created = append(result.Created, rel)
	return nil
}

func normalizeInitOptions(opts InitOptions) InitOptions {
	opts.ProjectPath = filepath.Clean(strings.TrimSpace(opts.ProjectPath))
	if opts.ProjectPath == "" {
		opts.ProjectPath = "."
	}
	opts.InitDir = strings.TrimSpace(opts.InitDir)
	opts.SID = strings.TrimSpace(opts.SID)
	opts.Label = strings.TrimSpace(opts.Label)
	opts.Name = strings.TrimSpace(opts.Name)
	opts.PackageName = strings.TrimSpace(opts.PackageName)
	if opts.PackageName == "" {
		opts.PackageName = filepath.Base(opts.ProjectPath)
	}
	if opts.Label == "" {
		opts.Label = strings.ToUpper(opts.SID)
	}
	if opts.Name == "" {
		opts.Name = opts.Label
	}
	opts.Module = strings.TrimSpace(opts.Module)
	if strings.TrimSpace(opts.KnowledgeID) == "" {
		opts.KnowledgeID = opts.SID
	}
	return opts
}

func resolveInitDir(projectPath, explicit string) (string, error) {
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("pp init: init-dir: %w", err)
		}
		if err := requireInitDir(abs); err != nil {
			return "", err
		}
		return abs, nil
	}
	start, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("pp init: project path: %w", err)
	}
	dir := start
	for {
		for _, cand := range []string{
			filepath.Join(filepath.Dir(dir), ".ang", "init"),
			filepath.Join(dir, ".ang", "init"),
		} {
			if err := requireInitDir(cand); err == nil {
				return cand, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("pp init: no .ang/init templates found from %s (consumer repo must ship them)", projectPath)
}

func requireInitDir(dir string) error {
	st, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("pp init: %s is not a directory", dir)
	}
	return nil
}

func initFuncMap() template.FuncMap {
	return template.FuncMap{
		"upper":  strings.ToUpper,
		"lower":  strings.ToLower,
		"pascal": toExportIdentifier,
	}
}

func toExportIdentifier(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, "")
}
