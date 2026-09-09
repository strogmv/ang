package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sdkManifestName marks a directory that an earlier build filled with the
// generated SDK. Its presence is the only positive proof that wiping the
// directory destroys nothing but our own output.
const sdkManifestName = "sdk-manifest.json"

// projectMarkers are files and directories that only a real project, never a
// generated SDK, contains at its top level.
var projectMarkers = []string{".git", "package.json", "src", "node_modules", "go.mod", "cue"}

// ensureSDKTarget refuses to treat appDir as the SDK output directory unless it
// is missing, empty, or was written by a previous build.
//
// copyFrontendSDK removes the target directory before filling it, and the flag
// takes an arbitrary path. Pointing it at the application checkout instead of
// its SDK subdirectory therefore deletes the whole frontend, source, history
// and all. That happened three times to one project before this check
// existed. The cost of the check is one directory listing.
func ensureSDKTarget(appDir string) error {
	target := strings.TrimSpace(appDir)
	if target == "" {
		return nil
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve sdk target %q: %w", target, err)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read sdk target %s: %w", abs, err)
	}
	if len(entries) == 0 {
		return nil
	}
	found := map[string]bool{}
	for _, entry := range entries {
		found[entry.Name()] = true
	}
	if found[sdkManifestName] {
		return nil
	}
	var markers []string
	for _, marker := range projectMarkers {
		if found[marker] {
			markers = append(markers, marker)
		}
	}
	if len(markers) > 0 {
		return fmt.Errorf("refusing to replace %s with the frontend SDK: it contains %s, so it looks like a project, not an SDK output directory; point --frontend-app-dir at the SDK subdirectory (for example %s)",
			abs, strings.Join(markers, ", "), filepath.Join(abs, "src", "@sdk"))
	}
	return fmt.Errorf("refusing to replace %s with the frontend SDK: it is not empty and has no %s from a previous build; delete it yourself if that is intended",
		abs, sdkManifestName)
}
