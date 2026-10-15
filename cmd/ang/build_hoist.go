package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/golit"
)

// hoistGoLiterals is the last rewrite of a Go backend's generated files: a
// function literal repeated across operations is declared once in golit.File,
// which also drops the imports only that literal used and renumbers the files'
// own //line directives. A build runs it on the staged backend (previousBackend
// is the same directory: the staged copy of the project holds the last
// build's literals file); a dry run, and so `ang build --check`, runs it
// through dryRunHoist with previousBackend = the project's backend. Both must
// leave the same bytes, or --check reports files a build would not change.
//
// It is syntax only (go/parser, go/format): no type information and no go
// build, so a dry run pays well under a second for it, not a go verify.
func hoistGoLiterals(backend, previousBackend string, logText func(string, ...any)) (golit.Stats, error) {
	st, err := golit.HoistTreeFrom(backend, previousBackend)
	if err != nil {
		return st, err
	}
	if st.Hoisted > 0 {
		logText("Hoisted %d repeated literals (%d copies): generated Go %d KB → %d KB", st.Hoisted, st.Replaced, st.Before/1024, st.After/1024)
	}
	return st, nil
}

// dryRunHoistResult is what hoisting changes besides the dry run's own files.
type dryRunHoistResult struct {
	// Rewrites are project files a build keeps without generating them (a
	// hand-written *.gen.go, a file a partial build does not remove) that
	// hoisting rewrites.
	Rewrites []dryRunFileChange
	// Deletions are literals files a build deletes: nothing is repeated any
	// more. golit.File is never in ang-generated.txt (hoisting, not an
	// emitter, writes it), so planGeneratedFiles cannot report them.
	Deletions []dryRunFileChange
}

// dryRunHoist hoists a dry run's backend the way a build hoists the staged
// project. There a package holds, next to what the build generated, the
// project's *.gen.go files it keeps (kept reports them): hoisting counts
// their literals and may rewrite them too. They are copied in for the pass,
// compared with the project afterwards and taken out again, so the dry run's
// file list stays what the emitters produced.
func dryRunHoist(backend, intendedBackend, projectPath string, kept func(abs string) bool, withDiff bool, logText func(string, ...any)) (dryRunHoistResult, error) {
	var result dryRunHoistResult
	type sibling struct{ staged, project string }
	var siblings []sibling
	err := filepath.WalkDir(backend, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if path != backend && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if matches, _ := filepath.Glob(filepath.Join(path, "*.gen.go")); len(matches) == 0 {
			return nil
		}
		rel, err := filepath.Rel(backend, path)
		if err != nil {
			return err
		}
		projectDir := filepath.Join(intendedBackend, rel)
		entries, err := os.ReadDir(projectDir)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".gen.go") || strings.HasSuffix(name, "_test.go") || name == golit.File {
				continue
			}
			staged, project := filepath.Join(path, name), filepath.Join(projectDir, name)
			if _, err := os.Lstat(staged); err == nil || !kept(project) {
				continue
			}
			data, err := os.ReadFile(project)
			if err != nil {
				return err
			}
			if err := os.WriteFile(staged, data, 0o644); err != nil {
				return err
			}
			siblings = append(siblings, sibling{staged: staged, project: project})
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	st, err := hoistGoLiterals(backend, intendedBackend, logText)
	if err != nil {
		return result, err
	}

	for _, s := range siblings {
		after, err := os.ReadFile(s.staged)
		if err != nil {
			return result, err
		}
		before, err := os.ReadFile(s.project)
		if err != nil {
			return result, err
		}
		if err := os.Remove(s.staged); err != nil {
			return result, err
		}
		if bytes.Equal(before, after) {
			continue
		}
		change := dryRunFileChange{Path: filepath.ToSlash(filepath.Clean(s.project)), Action: "update", Label: dryRunDiffLabel(projectPath, s.project)}
		if withDiff {
			change.Diff = unifiedFileDiff(change.Label, before, after, true)
		}
		result.Rewrites = append(result.Rewrites, change)
	}
	for _, pkg := range st.Retired {
		abs, err := filepath.Abs(filepath.Join(intendedBackend, filepath.FromSlash(pkg), golit.File))
		if err != nil {
			return result, err
		}
		result.Deletions = append(result.Deletions, dryRunFileChange{Path: filepath.ToSlash(abs), Action: "delete", Label: dryRunDiffLabel(projectPath, abs)})
	}
	sort.Slice(result.Rewrites, func(i, j int) bool { return result.Rewrites[i].Path < result.Rewrites[j].Path })
	return result, nil
}

// keptUngeneratedFile reports whether a build keeps a project file it did not
// generate: syncGeneratedFiles removes one only when the build is complete,
// ang-generated.txt lists it and it lies in a directory the build owns.
func keptUngeneratedFile(projectAbs, abs string, previous map[string]struct{}, ownedPaths []string, partial bool) bool {
	if partial {
		return true
	}
	rel, ok := projectRelativePath(projectAbs, abs)
	if !ok {
		return true
	}
	if _, listed := previous[rel]; !listed {
		return true
	}
	_, owned := ownedRootOf(abs, ownedPaths)
	return !owned
}
