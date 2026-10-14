package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
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
		"repo": {}, "events": {}, "events_meta": {}, "errors": {}, "project": {}, "projections": {},
		"infra": {}, "effects": {}, "schema": {}, "views": {}, "lint": {},
	}
	imported := importedCUEDirectories(basePath, cueRoot)
	var dead []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, ok := live[name]; ok || strings.HasPrefix(name, ".") {
			continue
		}
		// A package other CUE files import is a library (a project's own
		// definitions, next to ANG's vendored schema), not dead.
		if _, ok := imported[name]; ok {
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

var cueModuleRe = regexp.MustCompile(`(?m)^\s*module:\s*"([^"@]+)`)

// importedCUEDirectories returns the folders directly under the cue root that a
// .cue file there imports as "<module>/<cueRoot>/<folder>[/…]".
func importedCUEDirectories(basePath, cueRoot string) map[string]struct{} {
	out := map[string]struct{}{}
	data, err := os.ReadFile(filepath.Join(basePath, "cue.mod", "module.cue"))
	if err != nil {
		return out
	}
	m := cueModuleRe.FindSubmatch(data)
	if m == nil {
		return out
	}
	prefix := []byte(`"` + string(m[1]) + "/" + filepath.ToSlash(cueRoot) + "/")
	_ = filepath.WalkDir(filepath.Join(basePath, cueRoot), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".cue") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for rest := src; ; {
			i := bytes.Index(rest, prefix)
			if i < 0 {
				break
			}
			rest = rest[i+len(prefix):]
			end := bytes.IndexAny(rest, "/\"")
			if end > 0 {
				out[string(rest[:end])] = struct{}{}
			}
		}
		return nil
	})
	return out
}
