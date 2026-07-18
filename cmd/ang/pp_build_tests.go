package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func findGoModuleRoot(start string) (string, error) {
	dir := filepath.Clean(start)
	for {
		goMod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goMod); err == nil {
			return dir, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat %s: %w", goMod, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func runPaymentProviderGoTests(projectPath string) (status string, codes []string) {
	moduleRoot, err := findGoModuleRoot(projectPath)
	if err != nil {
		return "skipped", nil
	}
	rel, err := filepath.Rel(moduleRoot, filepath.Clean(projectPath))
	if err != nil || strings.HasPrefix(rel, "..") {
		return "skipped", nil
	}
	testTarget := "./" + filepath.ToSlash(rel) + "/..."
	cmd := exec.Command("go", "test", testTarget)
	cmd.Dir = moduleRoot
	configureBuildSubprocess(cmd)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		text := output.String()
		if strings.Contains(text, "build failed") {
			return "failed", []string{"GO_BUILD_FAILED"}
		}
		return "failed", []string{"GO_TEST_FAILED"}
	}
	return "passed", nil
}
