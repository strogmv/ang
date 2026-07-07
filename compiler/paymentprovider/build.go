package paymentprovider

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BuildOptions configures a payment provider code generation run.
type BuildOptions struct {
	ProjectPath  string
	CueRoot      string
	TemplatesDir string
	SchemaDir    string
	OutputDir    string // defaults to ProjectPath
}

// Build loads CUE intent, resolves field mappings, and emits Go sources.
func Build(opts BuildOptions) error {
	if opts.ProjectPath == "" {
		return fmt.Errorf("project path is required")
	}
	cueRoot := opts.CueRoot
	pc := LoadProjectConfig(opts.ProjectPath)
	if cueRoot == "" {
		cueRoot = pc.CueRoot
	}
	schemaDir := strings.TrimSpace(opts.SchemaDir)
	if schemaDir == "" && pc.SchemaDir != "" {
		resolved, err := ResolvePath(opts.ProjectPath, pc.SchemaDir)
		if err != nil {
			return err
		}
		schemaDir = resolved
	} else if schemaDir != "" {
		resolved, err := ResolvePath(opts.ProjectPath, schemaDir)
		if err != nil {
			return err
		}
		schemaDir = resolved
	}
	spec, err := Load(opts.ProjectPath, cueRoot, schemaDir)
	if err != nil {
		return err
	}
	data, err := BuildTemplateData(spec)
	if err != nil {
		return fmt.Errorf("build template data: %w", err)
	}
	tmplDir, err := ResolveTemplatesDir(opts.ProjectPath, opts.TemplatesDir)
	if err != nil {
		return err
	}
	outDir := opts.OutputDir
	if outDir == "" {
		outDir = opts.ProjectPath
	}
	if err := Emit(tmplDir, outDir, data); err != nil {
		return fmt.Errorf("emit: %w", err)
	}
	return nil
}

// BuildFromProject is a convenience wrapper using paths relative to project root.
func BuildFromProject(projectPath, cueRoot, templatesDir string) error {
	return Build(BuildOptions{
		ProjectPath:  filepath.Clean(projectPath),
		CueRoot:      cueRoot,
		TemplatesDir: templatesDir,
	})
}
