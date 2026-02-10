package mcp

import (
	"fmt"
	"path/filepath"
	"strings"
)

func validateCuePath(path string) error {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || clean == "" {
		return fmt.Errorf("invalid path")
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return fmt.Errorf("path escapes workspace")
	}
	if !strings.HasPrefix(clean, "cue"+string(filepath.Separator)) {
		return fmt.Errorf("path must be under cue/")
	}
	if filepath.Ext(clean) != ".cue" {
		return fmt.Errorf("path must be a .cue file")
	}
	return nil
}

func validateReadPath(path string) error {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || clean == "" {
		return fmt.Errorf("invalid path")
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return fmt.Errorf("path escapes workspace")
	}
	return nil
}
