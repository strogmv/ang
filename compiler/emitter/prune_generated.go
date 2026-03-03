package emitter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func fileContainsAny(path string, markers ...string) bool {
	if len(markers) == 0 {
		return true
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(b)
	for _, m := range markers {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}

func pruneGeneratedFiles(dir string, keep map[string]struct{}, shouldPrune func(name string) bool, verifyGenerated func(path string) bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Nothing to prune when directory does not exist.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if shouldPrune != nil && !shouldPrune(name) {
			continue
		}
		if _, ok := keep[name]; ok {
			continue
		}
		path := filepath.Join(dir, name)
		if verifyGenerated != nil && !verifyGenerated(path) {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Printf("Removed stale generated file: %s\n", path)
	}

	return nil
}
