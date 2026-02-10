package mcp

import (
	"os"
	"strings"
)

func resolveANGExecutable() string {
	if env := strings.TrimSpace(os.Getenv("ANG_BIN")); env != "" {
		return env
	}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return exe
	}
	return "./ang_bin"
}
