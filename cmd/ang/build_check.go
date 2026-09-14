package main

import (
	"path/filepath"
	"sort"
	"strings"
)

// dryRunDifferences lists, as project-relative slash paths, the generated
// files a dry run found would be created or changed. It is what `ang build
// --check` reports.
//
// A dry run only walks what generation produced, so a committed file that the
// generator no longer emits is not a difference here: that kind of staleness
// cannot be told apart from a hand-written file without a list of emitted
// files, which the generator does not keep.
func dryRunDifferences(man dryRunManifest, projectPath string) []dryRunFileChange {
	root, _ := filepath.Abs(projectPath)
	var differences []dryRunFileChange
	for _, target := range man.Targets {
		for _, change := range target.Changes {
			action := strings.ToLower(strings.TrimSpace(change.Action))
			if action != "create" && action != "update" {
				continue
			}
			path := filepath.FromSlash(change.Path)
			if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				path = rel
			}
			differences = append(differences, dryRunFileChange{Path: filepath.ToSlash(path), Action: action})
		}
	}
	sort.Slice(differences, func(i, j int) bool { return differences[i].Path < differences[j].Path })
	return differences
}
