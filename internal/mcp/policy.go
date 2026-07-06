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
	cr := resolveMCPCueRoot(".")
	if !strings.HasPrefix(clean, cr+string(filepath.Separator)) {
		return fmt.Errorf("path must be under %s/", cr)
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
