package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func ensureReleaseGoModule(targetDir, modulePath string) error {
	if strings.TrimSpace(targetDir) == "" {
		return fmt.Errorf("empty release target dir")
	}
	if strings.TrimSpace(modulePath) == "" {
		modulePath = "github.com/strogmv/ang"
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	goVersion := detectRootGoVersion("go.mod")
	modPath := filepath.Join(targetDir, "go.mod")
	if _, err := os.Stat(modPath); err == nil {
		return nil
	}

	// Release mode builds a separate Go module under dist/.
	// If we only generate `module` + `go` (without `require` block), `go build` fails
	// because Go can't resolve dependency versions without entries in go.mod.
	rootGoMod, err := os.ReadFile("go.mod")
	if err != nil {
		// Fallback: minimal go.mod (still better than failing with missing file).
		content := fmt.Sprintf("module %s\n\ngo %s\n", modulePath, goVersion)
		return os.WriteFile(modPath, []byte(content), 0o644)
	}

	content := string(rootGoMod)
	// Rewrite `module ...` to match the intended module path for the release module.
	reModuleLine := regexp.MustCompile(`(?m)^module\s+.+\s*$`)
	content = reModuleLine.ReplaceAllString(content, "module "+modulePath)
	// Rewrite `go ...` to match the detected Go version (keeps repo reproducibility).
	reGoLine := regexp.MustCompile(`(?m)^go\s+[0-9]+\.[0-9]+(\.[0-9]+)?\s*$`)
	content = reGoLine.ReplaceAllString(content, "go "+goVersion)

	if err := os.WriteFile(modPath, []byte(content), 0o644); err != nil {
		return err
	}

	// Copy go.sum if available to speed up builds/reducing network variance.
	if rootGoSum, err := os.ReadFile("go.sum"); err == nil {
		_ = os.WriteFile(filepath.Join(targetDir, "go.sum"), rootGoSum, 0o644)
	}

	return nil
}

func detectRootGoVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "1.25"
	}
	re := regexp.MustCompile(`(?m)^go\s+([0-9]+\.[0-9]+)\s*$`)
	m := re.FindStringSubmatch(string(data))
	if len(m) != 2 {
		return "1.25"
	}
	return m[1]
}
