package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ANG_FRONTEND_TSC controls frontend typecheck gate:
// - "" / "0" / "false": disabled
// - "1" / "true": run when tsconfig.json exists
// - "strict": require tsconfig.json presence and successful typecheck
func runFrontendTypecheckGate(frontendDirs []string) error {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("ANG_FRONTEND_TSC")))
	if mode == "" || mode == "0" || mode == "false" {
		return nil
	}
	strict := mode == "strict"

	seen := make(map[string]bool, len(frontendDirs))
	var dirs []string
	for _, d := range frontendDirs {
		if d == "" {
			continue
		}
		abs, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		abs = filepath.Clean(abs)
		if seen[abs] {
			continue
		}
		seen[abs] = true
		dirs = append(dirs, abs)
	}

	if len(dirs) == 0 {
		if strict {
			return fmt.Errorf("ANG_FRONTEND_TSC=strict but no frontend directories were produced")
		}
		return nil
	}

	checkable := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		cfg := filepath.Join(dir, "tsconfig.json")
		if _, err := os.Stat(cfg); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
			fmt.Printf("Frontend typecheck skipped (missing package.json): %s\n", dir)
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "node_modules")); err != nil {
			fmt.Printf("Frontend typecheck skipped (missing node_modules): %s\n", dir)
			continue
		}
		checkable = append(checkable, dir)
	}

	if len(checkable) == 0 {
		if strict {
			return fmt.Errorf("ANG_FRONTEND_TSC=strict but no frontend directories with tsconfig.json + package.json + node_modules were found")
		}
		return nil
	}

	tscPath, tscErr := resolveTSCBinary()
	if tscErr != nil {
		return tscErr
	}

	checked := 0
	for _, dir := range checkable {
		checked++
		cmd := exec.Command(tscPath, "--noEmit", "--pretty", "false")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("frontend typecheck failed in %s: %v\n%s", dir, err, string(out))
		}
		fmt.Printf("Frontend typecheck passed: %s\n", dir)
	}

	return nil
}

func resolveTSCBinary() (string, error) {
	if v := strings.TrimSpace(os.Getenv("ANG_TSC_BIN")); v != "" {
		return v, nil
	}
	if p, err := exec.LookPath("tsc"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("ANG_FRONTEND_TSC enabled but `tsc` binary is not available (install TypeScript or set ANG_TSC_BIN)")
}
