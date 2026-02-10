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
	content := fmt.Sprintf("module %s\n\ngo %s\n", modulePath, goVersion)
	return os.WriteFile(modPath, []byte(content), 0o644)
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
