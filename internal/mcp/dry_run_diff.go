package mcp

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// newDryRunDiffDir creates the directory `ang build --diff-out` writes patches
// into for one ang_dry_run call, and the function that removes it.
func newDryRunDiffDir() (string, func(), error) {
	dir, err := os.MkdirTemp("", "ang-mcp-dry-run-diff-*")
	if err != nil {
		return "", func() {}, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

// readDryRunPatches returns the patches in dir, ordered by path. The output
// excerpt of ang_dry_run is capped, and a regenerated SDK can produce megabytes
// of diff, so each patch is cut at perFileLimit bytes and, once totalLimit
// bytes have been returned, the remaining patches are listed without content.
func readDryRunPatches(dir string, perFileLimit, totalLimit int) []map[string]any {
	var paths []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".patch") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	patches := make([]map[string]any, 0, len(paths))
	total := 0
	for _, path := range paths {
		rel, _ := filepath.Rel(dir, path)
		entry := map[string]any{"path": strings.TrimSuffix(filepath.ToSlash(rel), ".patch")}
		if total >= totalLimit {
			entry["omitted"] = "diff budget exhausted; run ang build --dry-run --diff-out <dir> for every patch"
			patches = append(patches, entry)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			entry["error"] = err.Error()
			patches = append(patches, entry)
			continue
		}
		text := string(data)
		if len(text) > perFileLimit {
			text = text[:perFileLimit]
			entry["truncated"] = true
		}
		total += len(text)
		entry["patch"] = text
		patches = append(patches, entry)
	}
	return patches
}
