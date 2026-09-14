package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type dryRunFileChange struct {
	Path   string `json:"path"`
	Action string `json:"action"` // create|update|unchanged
	// Label is the path shown in a diff, relative to the project when possible.
	Label string `json:"-"`
	// Diff is a unified diff, filled only when --diff or --diff-out is set. It is
	// kept out of the JSON manifest, which lists every generated file.
	Diff string `json:"-"`
}

type dryRunTargetManifest struct {
	Target   string             `json:"target"`
	Lang     string             `json:"lang"`
	Backend  string             `json:"backend_dir"`
	Frontend string             `json:"frontend_dir"`
	Changes  []dryRunFileChange `json:"changes"`
}

type dryRunManifest struct {
	Status               string                 `json:"status"`
	TotalTargets         int                    `json:"total_targets"`
	TotalGenerated       int                    `json:"total_generated_files"`
	TotalCreate          int                    `json:"total_create"`
	TotalUpdate          int                    `json:"total_update"`
	TotalUnchanged       int                    `json:"total_unchanged"`
	Targets              []dryRunTargetManifest `json:"targets"`
	Notes                []string               `json:"notes"`
	OptionalStepsSkipped []string               `json:"optional_steps_skipped,omitempty"`
}

func buildDryRunChanges(generatedRoot, intendedRoot string) ([]dryRunFileChange, error) {
	return collectDryRunChanges(generatedRoot, intendedRoot, intendedRoot, false)
}

// collectDryRunChanges compares every file generation produced under
// generatedRoot with the file at the same place under intendedRoot. With
// withDiff, each created or changed file also carries a unified diff labelled
// relative to labelRoot.
func collectDryRunChanges(generatedRoot, intendedRoot, labelRoot string, withDiff bool) ([]dryRunFileChange, error) {
	changes := make([]dryRunFileChange, 0)
	info, err := os.Stat(generatedRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return changes, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("generated root is not directory: %s", generatedRoot)
	}

	err = filepath.WalkDir(generatedRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(generatedRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		genBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(intendedRoot, rel)
		action := "create"
		existing, readErr := os.ReadFile(dest)
		if readErr == nil {
			if string(existing) == string(genBytes) {
				action = "unchanged"
			} else {
				action = "update"
			}
		}
		change := dryRunFileChange{
			Path:   filepath.ToSlash(filepath.Clean(dest)),
			Action: action,
			Label:  dryRunDiffLabel(labelRoot, dest),
		}
		if withDiff && action != "unchanged" {
			change.Diff = unifiedFileDiff(change.Label, existing, genBytes, readErr == nil)
		}
		changes = append(changes, change)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Action == changes[j].Action {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Action < changes[j].Action
	})
	return changes, nil
}

func printDryRunManifest(man dryRunManifest) {
	b, _ := json.MarshalIndent(man, "", "  ")
	fmt.Println(string(b))
}

func summarizeDryRunManifest(man *dryRunManifest) {
	man.TotalTargets = len(man.Targets)
	for _, t := range man.Targets {
		for _, c := range t.Changes {
			man.TotalGenerated++
			switch strings.ToLower(c.Action) {
			case "create":
				man.TotalCreate++
			case "update":
				man.TotalUpdate++
			default:
				man.TotalUnchanged++
			}
		}
	}
}
