package main

import (
	"path/filepath"
	"sort"
	"strings"
)

// dryRunDifferences lists, as project-relative slash paths, what `ang build
// --check` reports: generated files a dry run found would be created or
// changed, files ang-generated.txt lists that are no longer generated (action
// "delete"), and ang-generated.txt itself when it is missing or out of date.
func dryRunDifferences(man dryRunManifest, projectPath string) []dryRunFileChange {
	root, _ := filepath.Abs(projectPath)
	var differences []dryRunFileChange
	add := func(change dryRunFileChange) {
		action := strings.ToLower(strings.TrimSpace(change.Action))
		if action != "create" && action != "update" && action != "delete" {
			return
		}
		path := filepath.FromSlash(change.Path)
		if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			path = rel
		}
		differences = append(differences, dryRunFileChange{Path: filepath.ToSlash(path), Action: action})
	}
	for _, target := range man.Targets {
		for _, change := range target.Changes {
			add(change)
		}
	}
	if man.GeneratedList != nil {
		add(*man.GeneratedList)
	}
	for _, change := range man.Deletions {
		add(change)
	}
	sort.Slice(differences, func(i, j int) bool { return differences[i].Path < differences[j].Path })
	return differences
}
