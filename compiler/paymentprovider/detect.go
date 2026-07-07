package paymentprovider

import (
	"os"
	"path/filepath"

	"github.com/strogmv/ang/compiler"
)

// IsProject reports whether projectPath is a payment-provider ANG project
// (has .cue/provider.cue and is not a full application with cue/project).
func IsProject(projectPath, cueRoot string) bool {
	if cueRoot == "" {
		cueRoot = compiler.DefaultCueRoot
	}
	cueDir := filepath.Join(projectPath, cueRoot)
	if _, err := os.Stat(filepath.Join(cueDir, "provider.cue")); err != nil {
		return false
	}
	if matches, _ := filepath.Glob(filepath.Join(cueDir, "project", "*.cue")); len(matches) > 0 {
		return false
	}
	if _, err := os.Stat(filepath.Join(cueDir, "project.cue")); err == nil {
		return false
	}
	return true
}
