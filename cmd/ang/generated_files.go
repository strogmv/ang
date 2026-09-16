package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// generatedFilesName is the committed list of every file the last build
// generated in the project.
//
// Generation writes into an empty directory, so a build knows exactly what it
// produced. What it cannot see from the output alone is the difference between
// a file it no longer produces and a hand-written file next to generated ones:
// neither is in the output. The list tells them apart. A listed file that is not
// produced any more is removed; anything never listed is left alone.
const generatedFilesName = "ang-generated.txt"

const generatedFilesHeader = "# Files ANG generated in this project, one per line. Do not edit: ang build\n" +
	"# rewrites the list and removes a listed file once the project no longer generates it.\n"

// generatedFilesSync reports what syncGeneratedFiles did.
type generatedFilesSync struct {
	// Created is set when the project had no list before this build.
	Created bool
	// Removed lists files that are no longer generated and were removed.
	Removed []string
	// Outside lists files that are no longer generated but lie outside the
	// directories a build manages, so they were kept (and stay listed).
	Outside []string
}

// readGeneratedFiles returns the paths in the project's list and whether the
// list exists.
func readGeneratedFiles(projectRoot string) (map[string]struct{}, bool, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, generatedFilesName))
	if os.IsNotExist(err) {
		return map[string]struct{}{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	paths := map[string]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rel, ok := cleanGeneratedRel(line)
		if !ok {
			return nil, true, fmt.Errorf("%s: %q is not a path inside the project", generatedFilesName, line)
		}
		paths[rel] = struct{}{}
	}
	return paths, true, scanner.Err()
}

func renderGeneratedFiles(paths map[string]struct{}) []byte {
	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	var b bytes.Buffer
	b.WriteString(generatedFilesHeader)
	for _, p := range sorted {
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// cleanGeneratedRel normalizes a project-relative path to slash form and
// rejects paths that leave the project.
func cleanGeneratedRel(p string) (string, bool) {
	p = path.Clean(filepath.ToSlash(strings.TrimSpace(p)))
	if p == "." || p == ".." || strings.HasPrefix(p, "../") || path.IsAbs(p) {
		return "", false
	}
	return p, true
}

// projectRelativePath returns p relative to projectAbs in slash form, or false
// when p lies outside the project.
func projectRelativePath(projectAbs, p string) (string, bool) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(projectAbs, abs)
	if err != nil {
		return "", false
	}
	return cleanGeneratedRel(rel)
}

// ownedRelRoots turns the paths a build transaction owns into project-relative
// roots; owned paths outside the project are dropped.
func ownedRelRoots(projectAbs string, ownedPaths []string) []string {
	roots := make([]string, 0, len(ownedPaths))
	for _, owned := range ownedPaths {
		if rel, ok := projectRelativePath(projectAbs, owned); ok {
			roots = append(roots, rel)
		}
	}
	return roots
}

func ownedRootOf(rel string, roots []string) (string, bool) {
	for _, root := range roots {
		if rel == root || strings.HasPrefix(rel, root+"/") {
			return root, true
		}
	}
	return "", false
}

func staleGeneratedFiles(previous, produced map[string]struct{}) []string {
	var stale []string
	for p := range previous {
		if _, ok := produced[p]; !ok {
			stale = append(stale, p)
		}
	}
	sort.Strings(stale)
	return stale
}

func copyGeneratedSet(paths map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(paths))
	for p := range paths {
		out[p] = struct{}{}
	}
	return out
}

// applyGeneratedOutput copies every file generation wrote under genRoot to the
// same place under destRoot, the staged project, and records it in produced as
// a path relative to the project; intendedRoot is where destRoot stands in the
// project. Files that are already identical are not rewritten.
func applyGeneratedOutput(genRoot, destRoot, intendedRoot, projectRoot string, produced map[string]struct{}) error {
	info, err := os.Stat(genRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("generation output %s is not a directory", genRoot)
	}
	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	intendedAbs, err := filepath.Abs(intendedRoot)
	if err != nil {
		return err
	}
	return filepath.WalkDir(genRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(genRoot, p)
		if err != nil {
			return err
		}
		if projectRel, ok := projectRelativePath(projectAbs, filepath.Join(intendedAbs, rel)); ok {
			produced[projectRel] = struct{}{}
		}
		dest := filepath.Join(destRoot, rel)
		same, err := sameTransactionNodes(p, dest)
		if err != nil || same {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			if err := os.RemoveAll(dest); err != nil {
				return err
			}
			return copyTransactionPath(p, dest)
		}
		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		return copyTransactionFile(p, dest, fileInfo.Mode())
	})
}

// syncGeneratedFiles removes from the staged project every file the previous
// build generated and this one did not, then writes the new list there. A
// partial build (some targets or parts skipped) removes nothing and only adds
// to the list, since what it did not produce may belong to what it skipped.
func syncGeneratedFiles(projectRoot, workspace string, ownedPaths []string, produced map[string]struct{}, partial bool) (generatedFilesSync, error) {
	var result generatedFilesSync
	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return result, err
	}
	previous, hadList, err := readGeneratedFiles(projectAbs)
	if err != nil {
		return result, err
	}
	result.Created = !hadList
	next := copyGeneratedSet(produced)
	if partial {
		for p := range previous {
			next[p] = struct{}{}
		}
	} else {
		roots := ownedRelRoots(projectAbs, ownedPaths)
		for _, rel := range staleGeneratedFiles(previous, produced) {
			owner, owned := ownedRootOf(rel, roots)
			if !owned {
				if _, err := os.Lstat(filepath.Join(projectAbs, filepath.FromSlash(rel))); err == nil {
					result.Outside = append(result.Outside, rel)
					next[rel] = struct{}{}
				}
				continue
			}
			removed, err := removeFromWorkspace(workspace, rel, owner)
			if err != nil {
				return result, fmt.Errorf("remove %s: %w", rel, err)
			}
			if removed {
				result.Removed = append(result.Removed, rel)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, generatedFilesName), renderGeneratedFiles(next), 0o644); err != nil {
		return result, fmt.Errorf("write %s: %w", generatedFilesName, err)
	}
	return result, nil
}

// removeFromWorkspace deletes rel from the staged project and the directories
// that removal emptied, up to the owned root. Owned roots are real copies in
// the workspace; a path that resolves elsewhere through a symlink is refused.
func removeFromWorkspace(workspace, rel, owner string) (bool, error) {
	target := filepath.Join(workspace, filepath.FromSlash(rel))
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return false, nil
	}
	stop := filepath.Join(workspace, filepath.FromSlash(owner))
	if rel != owner {
		realStop, err := filepath.EvalSymlinks(stop)
		if err != nil {
			return false, err
		}
		realParent, err := filepath.EvalSymlinks(filepath.Dir(target))
		if err != nil {
			return false, err
		}
		if realParent != realStop && !strings.HasPrefix(realParent, realStop+string(filepath.Separator)) {
			return false, fmt.Errorf("%s resolves outside %s", rel, owner)
		}
	}
	if err := os.Remove(target); err != nil {
		return false, err
	}
	for dir := filepath.Dir(target); strings.HasPrefix(dir, stop+string(filepath.Separator)); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
	return true, nil
}

// planGeneratedFiles adds to a dry-run manifest what a build would do besides
// writing generated files: remove listed files that are no longer generated
// and bring ang-generated.txt up to date.
func planGeneratedFiles(projectRoot string, ownedPaths []string, man *dryRunManifest, partial bool) error {
	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	produced := map[string]struct{}{}
	for _, target := range man.Targets {
		for _, change := range target.Changes {
			if rel, ok := projectRelativePath(projectAbs, filepath.FromSlash(change.Path)); ok {
				produced[rel] = struct{}{}
			}
		}
	}
	previous, _, err := readGeneratedFiles(projectAbs)
	if err != nil {
		return err
	}
	next := copyGeneratedSet(produced)
	if partial {
		for p := range previous {
			next[p] = struct{}{}
		}
	} else {
		roots := ownedRelRoots(projectAbs, ownedPaths)
		for _, rel := range staleGeneratedFiles(previous, produced) {
			abs := filepath.Join(projectAbs, filepath.FromSlash(rel))
			info, err := os.Lstat(abs)
			if err != nil || info.IsDir() {
				continue
			}
			if _, owned := ownedRootOf(rel, roots); !owned {
				next[rel] = struct{}{}
				continue
			}
			man.Deletions = append(man.Deletions, dryRunFileChange{Path: filepath.ToSlash(abs), Action: "delete", Label: rel})
		}
	}
	listPath := filepath.Join(projectAbs, generatedFilesName)
	action := "create"
	if have, err := os.ReadFile(listPath); err == nil {
		action = "update"
		if bytes.Equal(have, renderGeneratedFiles(next)) {
			action = "unchanged"
		}
	}
	man.GeneratedList = &dryRunFileChange{Path: filepath.ToSlash(listPath), Action: action, Label: generatedFilesName}
	return nil
}
