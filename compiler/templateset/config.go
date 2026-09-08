package templateset

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler"
	"gopkg.in/yaml.v3"
)

// Config is the generic project file: which CUE to load, which templates to
// render, and how to name the output files. It carries no domain types.
type Config struct {
	CueRoot      string
	TemplatesDir string
	SchemaDir    string
	// ModuleDirs lists template libraries to parse alongside every template,
	// earliest first. A later library (or the set's own modules/) redefines a
	// block and wins.
	ModuleDirs []string
	// Outputs maps a template file to the file it produces. "{package}" expands
	// to the data's PackageName field when present. When empty, the caller
	// owns the file layout.
	Outputs map[string]string

	ExpertKnowledgeID string
	ExpertRoot        string
}

// LoadConfig reads ang.yaml from projectPath. Missing or unreadable files
// yield the compiler defaults rather than an error: a project without a
// config is still a valid template set.
func LoadConfig(projectPath string) Config {
	base := strings.TrimSpace(projectPath)
	if base == "" {
		base = "."
	}
	type angYAML struct {
		CueRoot           string            `yaml:"cue_root"`
		TemplatesDir      string            `yaml:"templates_dir"`
		SchemaDir         string            `yaml:"schema_dir"`
		ModuleDirs        []string          `yaml:"module_dirs"`
		Outputs           map[string]string `yaml:"outputs"`
		ExpertKnowledgeID string            `yaml:"expert_knowledge_id"`
		ExpertRoot        string            `yaml:"expert_root"`
	}
	defaults := Config{
		CueRoot:      compiler.DefaultCueRoot,
		TemplatesDir: "templates",
	}
	raw, err := os.ReadFile(filepath.Join(base, "ang.yaml"))
	if err != nil {
		return defaults
	}
	var cfg angYAML
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return defaults
	}
	pc := Config{
		CueRoot:           strings.TrimSpace(cfg.CueRoot),
		TemplatesDir:      strings.TrimSpace(cfg.TemplatesDir),
		SchemaDir:         strings.TrimSpace(cfg.SchemaDir),
		ModuleDirs:        trimAll(cfg.ModuleDirs),
		Outputs:           cfg.Outputs,
		ExpertKnowledgeID: strings.TrimSpace(cfg.ExpertKnowledgeID),
		ExpertRoot:        strings.TrimSpace(cfg.ExpertRoot),
	}
	if pc.CueRoot == "" {
		pc.CueRoot = compiler.DefaultCueRoot
	}
	if pc.TemplatesDir == "" {
		pc.TemplatesDir = "templates"
	}
	return pc
}

// ResolvePath resolves p against projectPath when p is relative.
func ResolvePath(projectPath, p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	return filepath.Clean(filepath.Join(projectPath, p)), nil
}

// ResolveTemplatesDir resolves templates_dir relative to projectPath.
func ResolveTemplatesDir(projectPath, templatesDir string) (string, error) {
	templatesDir = strings.TrimSpace(templatesDir)
	if templatesDir == "" {
		return "", errEmptyTemplatesDir
	}
	return ResolvePath(projectPath, templatesDir)
}

func trimAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
