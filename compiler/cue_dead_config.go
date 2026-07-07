package compiler

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

const codeCUEDeadDirectory = "CUE_DEAD_DIRECTORY"

func emitDeadCUEDirectoryDiagnostics(basePath, cueRoot string, opts PipelineOptions) {
	root := filepath.Join(basePath, cueRoot)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	live := map[string]struct{}{
		"domain": {}, "architecture": {}, "api": {}, "policy": {}, "policies": {},
		"repo": {}, "events": {}, "errors": {}, "project": {}, "projections": {},
		"infra": {}, "effects": {}, "schema": {}, "views": {}, "lint": {},
	}
	var dead []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, ok := live[name]; ok || strings.HasPrefix(name, ".") {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(root, name, "*.cue"))
		if len(files) > 0 {
			dead = append(dead, name)
		}
	}
	sort.Strings(dead)
	for _, name := range dead {
		files, _ := filepath.Glob(filepath.Join(root, name, "*.cue"))
		diag := normalizer.Warning{
			Kind:     "cue",
			Code:     codeCUEDeadDirectory,
			Severity: "warn",
			Message:  filepath.ToSlash(filepath.Join(cueRoot, name)) + " is not loaded by the ANG compiler",
			File:     files[0],
			Line:     1,
			Column:   1,
			Hint:     "Move the declarations into the closest loaded package (for schedules use " + filepath.ToSlash(filepath.Join(cueRoot, "api")) + ").",
		}
		LatestDiagnostics = append(LatestDiagnostics, diag)
		if opts.WarningSink != nil {
			opts.WarningSink(diag)
		}
	}
}
