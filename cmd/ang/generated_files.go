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
// generated for this project.
//
// Generation writes into an empty directory, so a build knows exactly what it
// produced. What it cannot see from the output alone is the difference between
// a file it no longer produces and a hand-written file next to generated ones:
// neither is in the output. The list tells them apart. A listed file that is not
// produced any more is removed; anything never listed is left alone.
//
// Entries are slash paths relative to the project root. They may lead out of it
// (`../app/src/@sdk/index.ts`) because the frontend SDK is generated into the
// application repository; only directories the build owns are ever deleted from.
const generatedFilesName = "ang-generated.txt"

const generatedFilesHeader = "# Files ANG generated for this project, one per line. Do not edit: ang build\n" +
	"# rewrites the list and removes a listed file once the project no longer generates it.\n"

// generatedFilesSync reports what syncGeneratedFiles did.
type generatedFilesSync struct {
	// Created is set when the project had no list before this build.
	Created bool
	// Removed lists files that are no longer generated and were removed.
	Removed []string
	// Outside lists files that are no longer generated but lie outside the
	// directories a build owns, so they were kept (and stay listed).
	Outside []string
}

// stagedPathFunc maps a path in the project to where the running build stages
// it, so a build changes its own workspace and not the project.
type stagedPathFunc func(path string) string

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
			return nil, true, fmt.Errorf("%s: %q is not a path relative to the project", generatedFilesName, line)
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

// cleanGeneratedRel normalizes a path to slash form. Absolute paths are
// rejected; a path that leaves the project is not, since generated output may
// live in a sibling repository.
func cleanGeneratedRel(p string) (string, bool) {
	p = path.Clean(filepath.ToSlash(strings.TrimSpace(p)))
	if p == "." || p == ".." || path.IsAbs(p) || strings.HasPrefix(p, "/") {
		return "", false
	}
	return p, true
}

// projectRelativePath returns p relative to projectAbs in slash form.
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

// generatedAbsPath is the inverse of projectRelativePath.
func generatedAbsPath(projectAbs, rel string) string {
	return filepath.Clean(filepath.Join(projectAbs, filepath.FromSlash(rel)))
}

// ownedRootOf names the build-owned directory (or file) holding abs, if any.
// Only these are ever deleted from: elsewhere a build has no claim, and the
// staged project links straight into the real one.
func ownedRootOf(abs string, ownedPaths []string) (string, bool) {
	abs = filepath.Clean(abs)
	for _, owned := range ownedPaths {
		owned = filepath.Clean(owned)
		if abs == owned || strings.HasPrefix(abs, owned+string(filepath.Separator)) {
			return owned, true
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
func syncGeneratedFiles(projectRoot string, staged stagedPathFunc, ownedPaths []string, produced map[string]struct{}, partial bool) (generatedFilesSync, error) {
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
		for _, rel := range staleGeneratedFiles(previous, produced) {
			abs := generatedAbsPath(projectAbs, rel)
			owner, owned := ownedRootOf(abs, ownedPaths)
			if !owned {
				if _, err := os.Lstat(abs); err == nil {
					result.Outside = append(result.Outside, rel)
					next[rel] = struct{}{}
				}
				continue
			}
			removed, err := removeStagedFile(staged(abs), staged(owner))
			if err != nil {
				return result, fmt.Errorf("remove %s: %w", rel, err)
			}
			if removed {
				result.Removed = append(result.Removed, rel)
			}
		}
	}
	listPath := staged(filepath.Join(projectAbs, generatedFilesName))
	if err := os.MkdirAll(filepath.Dir(listPath), 0o755); err != nil {
		return result, fmt.Errorf("create directory for %s: %w", generatedFilesName, err)
	}
	if err := os.WriteFile(listPath, renderGeneratedFiles(next), 0o644); err != nil {
		return result, fmt.Errorf("write %s: %w", generatedFilesName, err)
	}
	return result, nil
}

// removeStagedFile deletes one staged file and the directories its removal
// emptied, up to the staged owned root. An owned root is a real copy in the
// build's workspace; a path that resolves out of it through a symlink is
// refused rather than followed into the project.
func removeStagedFile(target, ownerRoot string) (bool, error) {
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
	if target != ownerRoot {
		realRoot, err := filepath.EvalSymlinks(ownerRoot)
		if err != nil {
			return false, err
		}
		realParent, err := filepath.EvalSymlinks(filepath.Dir(target))
		if err != nil {
			return false, err
		}
		if realParent != realRoot && !strings.HasPrefix(realParent, realRoot+string(filepath.Separator)) {
			return false, fmt.Errorf("%s resolves outside %s", target, ownerRoot)
		}
	}
	if err := os.Remove(target); err != nil {
		return false, err
	}
	for dir := filepath.Dir(target); strings.HasPrefix(dir, ownerRoot+string(filepath.Separator)); dir = filepath.Dir(dir) {
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
		for _, rel := range staleGeneratedFiles(previous, produced) {
			abs := generatedAbsPath(projectAbs, rel)
			info, err := os.Lstat(abs)
			if err != nil || info.IsDir() {
				continue
			}
			if _, owned := ownedRootOf(abs, ownedPaths); !owned {
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
