package paymentprovider

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler"
	"gopkg.in/yaml.v3"
)

// ProjectConfig holds optional overrides from ang.yaml in a provider package.
type ProjectConfig struct {
	CueRoot      string
	TemplatesDir string
	SchemaDir    string
}

// LoadProjectConfig reads ang.yaml from projectPath (provider package root).
func LoadProjectConfig(projectPath string) ProjectConfig {
	base := strings.TrimSpace(projectPath)
	if base == "" {
		base = "."
	}
	type angYAML struct {
		CueRoot      string `yaml:"cue_root"`
		TemplatesDir string `yaml:"templates_dir"`
		SchemaDir    string `yaml:"schema_dir"`
	}
	defaults := ProjectConfig{
		CueRoot:      compiler.DefaultCueRoot,
		TemplatesDir: "templates",
	}
	data, err := os.ReadFile(filepath.Join(base, "ang.yaml"))
	if err != nil {
		return defaults
	}
	var cfg angYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return defaults
	}
	pc := ProjectConfig{
		CueRoot:      strings.TrimSpace(cfg.CueRoot),
		TemplatesDir: strings.TrimSpace(cfg.TemplatesDir),
		SchemaDir:    strings.TrimSpace(cfg.SchemaDir),
	}
	if pc.CueRoot == "" {
		pc.CueRoot = compiler.DefaultCueRoot
	}
	if pc.TemplatesDir == "" {
		pc.TemplatesDir = "templates"
	}
	return pc
}

// ResolveSchemaDir returns the absolute schema directory for CUE loading.
// When SchemaDir is empty, schema is expected at <project>/<cueRoot>/schema/.
func (pc ProjectConfig) ResolveSchemaDir(projectPath, cueRoot string) (string, error) {
	if strings.TrimSpace(pc.SchemaDir) == "" {
		if cueRoot == "" {
			cueRoot = pc.CueRoot
		}
		if cueRoot == "" {
			cueRoot = compiler.DefaultCueRoot
		}
		return filepath.Join(projectPath, cueRoot, "schema"), nil
	}
	return ResolvePath(projectPath, pc.SchemaDir)
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
