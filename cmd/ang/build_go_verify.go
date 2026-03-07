package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var reGoBuildLine = regexp.MustCompile(`^((?:\./)?internal/[^:]+|/.*/internal/[^:]+):([0-9]+):([0-9]+):\s*(.+)$`)

func runGeneratedGoVerify(backends []string) error {
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
				fmt.Printf("Skipping go verify (go.mod not found): %s\n", cleaned)
				continue
			}
			return fmt.Errorf("check go.mod in %s: %w", cleaned, err)
		}

		fmt.Printf("Running go verify: go build ./internal/... (dir=%s)\n", cleaned)
		cmd := exec.Command("go", "build", "./internal/...")
		cmd.Dir = cleaned
		out, err := cmd.CombinedOutput()
		if err != nil {
			raw := strings.TrimSpace(string(out))
			mapped := mapGoBuildErrorsToCUESource(raw, cleaned)
			if len(mapped) == 0 {
				return fmt.Errorf("go verify failed in %s:\n%s", cleaned, raw)
			}
			return fmt.Errorf("go verify failed in %s:\n%s", cleaned, strings.Join(mapped, "\n"))
		}
	}
	return nil
}

func mapGoBuildErrorsToCUESource(goBuildOutput string, projectPath string) []string {
	if strings.TrimSpace(goBuildOutput) == "" {
		return nil
	}
	var out []string
	sc := bufio.NewScanner(strings.NewReader(goBuildOutput))
	for sc.Scan() {
		line := sc.Text()
		m := reGoBuildLine.FindStringSubmatch(line)
		if len(m) == 0 {
			if strings.TrimSpace(line) != "" {
				out = append(out, line)
			}
			continue
		}
		fileRef := strings.TrimSpace(m[1])
		lineNo, _ := strconv.Atoi(m[2])
		col := strings.TrimSpace(m[3])
		msg := strings.TrimSpace(m[4])
		filePath := resolveBuildPath(projectPath, fileRef)
		if cueRef := findGeneratedFromComment(filePath, lineNo); cueRef != "" {
			out = append(out, fmt.Sprintf("%s -> %s:%d:%s: %s", cueRef, filepath.ToSlash(fileRef), lineNo, col, msg))
			continue
		}
		out = append(out, line)
	}
	return out
}

func resolveBuildPath(projectPath, fileRef string) string {
	ref := strings.TrimSpace(fileRef)
	if ref == "" {
		return ""
	}
	if filepath.IsAbs(ref) {
		return filepath.Clean(ref)
	}
	return filepath.Clean(filepath.Join(projectPath, ref))
}

func findGeneratedFromComment(filePath string, line int) string {
	if line <= 0 || strings.TrimSpace(filePath) == "" {
		return ""
	}
	b, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) == 0 {
		return ""
	}
	idx := line - 1
	if idx >= len(lines) {
		idx = len(lines) - 1
	}
	start := idx - 80
	if start < 0 {
		start = 0
	}
	for i := idx; i >= start; i-- {
		s := strings.TrimSpace(lines[i])
		if strings.HasPrefix(s, "// Generated from:") {
			return strings.TrimSpace(strings.TrimPrefix(s, "// Generated from:"))
		}
	}
	return ""
}
