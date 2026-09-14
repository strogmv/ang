package main

import "path/filepath"

// realPath returns path absolute with symlinks resolved, or the absolute path
// when it cannot be resolved (it may not exist yet).
func realPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}
