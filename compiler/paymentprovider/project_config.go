package paymentprovider

import (
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/templateset"
)

// ProjectConfig is the payment-provider view of a generic template-set config.
// Domain mapping still lives in this package; file layout comes from templateset.
type ProjectConfig struct {
	templateset.Config
}

// LoadProjectConfig reads ang.yaml from projectPath.
func LoadProjectConfig(projectPath string) ProjectConfig {
	return ProjectConfig{Config: templateset.LoadConfig(projectPath)}
}

// ResolvePath resolves p against projectPath when p is relative.
func ResolvePath(projectPath, p string) (string, error) {
	return templateset.ResolvePath(projectPath, p)
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
	return templateset.ResolvePath(projectPath, pc.SchemaDir)
}
