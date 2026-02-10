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
		if existing, err := os.ReadFile(dest); err == nil {
			if string(existing) == string(genBytes) {
				action = "unchanged"
			} else {
				action = "update"
			}
		}
		changes = append(changes, dryRunFileChange{
			Path:   filepath.ToSlash(filepath.Clean(dest)),
			Action: action,
		})
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
