package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runGeneratedGoTests(backends []string) error {
	seen := make(map[string]struct{}, len(backends))
	for _, backend := range backends {
		dir := strings.TrimSpace(backend)
		if dir == "" {
			continue
		}
		cleaned := filepath.Clean(dir)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}

		goModPath := filepath.Join(cleaned, "go.mod")
		if _, err := os.Stat(goModPath); err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("Skipping go test (go.mod not found): %s\n", cleaned)
				continue
			}
			return fmt.Errorf("check go.mod in %s: %w", cleaned, err)
		}

		fmt.Printf("Running tests: go test ./... (dir=%s)\n", cleaned)
		cmd := exec.Command("go", "test", "./...")
		cmd.Dir = cleaned
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go test failed in %s: %w", cleaned, err)
		}
	}
	return nil
}
