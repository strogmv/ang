package paymentprovider

import (
	"fmt"
	"os"
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
	_, err := BuildWithResult(opts)
	return err
}

// BuildWithResult runs generation and returns the generator-owned output manifest.
func BuildWithResult(opts BuildOptions) (BuildResult, error) {
	if opts.ProjectPath == "" {
		return BuildResult{}, fmt.Errorf("project path is required")
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
			return BuildResult{}, err
		}
		schemaDir = resolved
	} else if schemaDir != "" {
		resolved, err := ResolvePath(opts.ProjectPath, schemaDir)
		if err != nil {
			return BuildResult{}, err
		}
		schemaDir = resolved
	}
	spec, err := Load(opts.ProjectPath, cueRoot, schemaDir)
	if err != nil {
		return BuildResult{}, err
	}
	for _, iss := range Vet(spec) {
		if iss.Severity == "error" && (iss.Code == "PP012" || iss.Code == "PP013") {
			return BuildResult{}, fmt.Errorf("%s: %s", iss.Code, iss.Message)
		}
	}
	data, err := BuildTemplateData(spec)
	if err != nil {
		return BuildResult{}, fmt.Errorf("build template data: %w", err)
	}
	tmplDir, err := ResolveTemplatesDir(opts.ProjectPath, opts.TemplatesDir)
	if err != nil {
		return BuildResult{}, err
	}
	if usesV1Datatypes(tmplDir) && hasNestedRequestObjects(data) {
		return BuildResult{}, fmt.Errorf("nested request objects require the v2 template set")
	}
	outDir := opts.OutputDir
	if outDir == "" {
		outDir = opts.ProjectPath
	}
	moduleDirs := make([]string, 0, len(pc.ModuleDirs))
	for _, dir := range pc.ModuleDirs {
		resolved, err := ResolvePath(opts.ProjectPath, dir)
		if err != nil {
			return BuildResult{}, err
		}
		moduleDirs = append(moduleDirs, resolved)
	}
	files, err := EmitWithOptions(tmplDir, outDir, data, EmitOptions{
		ModuleDirs: moduleDirs,
		Outputs:    pc.Outputs,
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("emit: %w", err)
	}
	return BuildResult{OutputDir: outDir, Files: files}, nil
}

// BuildFromProject is a convenience wrapper using paths relative to project root.
func BuildFromProject(projectPath, cueRoot, templatesDir string) error {
	return Build(BuildOptions{
		ProjectPath:  filepath.Clean(projectPath),
		CueRoot:      cueRoot,
		TemplatesDir: templatesDir,
	})
}

func usesV1Datatypes(templatesDir string) bool {
	_, err := os.Stat(filepath.Join(templatesDir, "datatypes.go.tmpl"))
	return err == nil
}

func hasNestedRequestObjects(data *TemplateData) bool {
	if data == nil {
		return false
	}
	check := func(def *ResolvedRequestDef) bool {
		return def != nil && len(def.ObjectTypes) > 0
	}
	return check(data.PayinRequest) || check(data.PayoutRequest) ||
		check(data.PayinStatusRequest) || check(data.PayoutStatusRequest) ||
		check(data.RefundRequest) || check(data.P2PRequest)
}
