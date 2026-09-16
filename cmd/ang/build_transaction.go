package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
)

// buildBeforeCommitHook is a test seam: it runs after generation and verification
// have finished in the workspace and just before generated output is published,
// which is the widest window in which someone may still be editing the project.
var buildBeforeCommitHook func(projectPath string)

// A build transaction publishes generated output with a three-way, file-level
// merge instead of swapping whole directories.
//
// The transaction owns directories such as internal/, db/ and tests/, but those
// directories also hold hand-written files (migrations, e2e tests, deploy
// manifests), and a build takes minutes. Replacing a directory with a snapshot
// taken at the start of the build silently discarded everything written into it
// meanwhile. Now each file is decided on its own:
//
//	base  — the project as it was when the transaction began
//	stage — what generation produced
//	path  — the project as it is at commit time
//
// A file the generator did not change is left exactly as it now is in the
// project, whatever happened to it during the build. A file the generator did
// change is published; if it was also changed in the project during the build,
// the project's version is first saved to the conflict directory.
type buildTransactionEntry struct {
	path    string
	base    string
	stage   string
	backup  string
	workDir string
	existed bool
	// staged means stage holds generated output for this path: either a
	// StagePath was handed out for it or a workspace was captured into it.
	// An entry that was never staged has nothing to publish.
	staged bool
}

type commitJournalOp struct {
	path      string
	backup    string
	published bool
}

type buildTransaction struct {
	entries      []buildTransactionEntry
	workspaces   []string
	journal      []commitJournalOp
	stageErr     error
	conflictRoot string
	conflictDir  string
	conflicts    []string
	swept        buildScratchSweep
	keepWorkDirs bool
	done         bool
}

func beginBuildTransaction(paths []string) (*buildTransaction, error) {
	tx := &buildTransaction{}
	cleaned := compactTransactionPaths(paths)
	sweptParents := map[string]struct{}{}
	for _, path := range cleaned {
		parent := filepath.Dir(path)
		if _, done := sweptParents[parent]; !done {
			sweptParents[parent] = struct{}{}
			sweepAbandonedBuildScratch(parent, transactionScratchPrefix, time.Now(), &tx.swept)
		}
	}
	for _, path := range cleaned {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tx.cleanupWorkDirs()
			return nil, fmt.Errorf("create transaction parent for %s: %w", path, err)
		}
		workDir, err := os.MkdirTemp(filepath.Dir(path), transactionScratchPrefix+"*")
		if err != nil {
			tx.cleanupWorkDirs()
			return nil, fmt.Errorf("create build transaction for %s: %w", path, err)
		}
		// Best effort: without an owner the directory is only aged out.
		_ = writeBuildScratchOwner(workDir, false)
		entry := buildTransactionEntry{
			path:    path,
			base:    filepath.Join(workDir, "base"),
			stage:   filepath.Join(workDir, "stage"),
			backup:  filepath.Join(workDir, "backup"),
			workDir: workDir,
		}
		if _, err := os.Lstat(path); err == nil {
			entry.existed = true
			if err := copyTransactionPath(path, entry.base); err != nil {
				_ = os.RemoveAll(workDir)
				tx.cleanupWorkDirs()
				return nil, fmt.Errorf("snapshot %s: %w", path, err)
			}
		} else if !os.IsNotExist(err) {
			_ = os.RemoveAll(workDir)
			tx.cleanupWorkDirs()
			return nil, fmt.Errorf("inspect %s: %w", path, err)
		}
		tx.entries = append(tx.entries, entry)
	}
	return tx, nil
}

// SetConflictDir sets where project-side versions of files that were edited
// during the build and also regenerated are saved. root is the directory the
// saved paths are made relative to. Without it, a directory next to each
// entry is used.
func (tx *buildTransaction) SetConflictDir(root, dir string) {
	if tx == nil {
		return
	}
	tx.conflictRoot = root
	tx.conflictDir = dir
}

// ConflictDir returns the directory conflict copies were saved to, if any.
func (tx *buildTransaction) ConflictDir() string {
	if tx == nil {
		return ""
	}
	return tx.conflictDir
}

// Swept reports the abandoned scratch directories removed while this
// transaction set up, and those kept because they hold originals.
func (tx *buildTransaction) Swept() buildScratchSweep {
	if tx == nil {
		return buildScratchSweep{}
	}
	return tx.swept
}

// Conflicts lists, relative to the conflict root, every file whose in-project
// edit was set aside because generation replaced it.
func (tx *buildTransaction) Conflicts() []string {
	if tx == nil {
		return nil
	}
	return append([]string(nil), tx.conflicts...)
}

func (tx *buildTransaction) Commit() error {
	if tx == nil || tx.done {
		return nil
	}
	// Refuse before touching anything: a stage that could not be prepared would
	// read as "the generator deleted every file" and remove real ones.
	if tx.stageErr != nil {
		return fmt.Errorf("prepare staged output: %w", tx.stageErr)
	}
	for i := range tx.entries {
		if err := tx.mergeEntry(&tx.entries[i]); err != nil {
			if undoErr := tx.undoJournal(); undoErr != nil {
				return fmt.Errorf("%w; restoring the project also failed, originals are kept under %s: %v", err, tx.entries[i].workDir, undoErr)
			}
			return err
		}
	}
	tx.done = true
	tx.cleanupWorkDirs()
	return nil
}

func (tx *buildTransaction) Rollback() error {
	if tx == nil || tx.done {
		return nil
	}
	tx.done = true
	tx.cleanupWorkDirs()
	return nil
}

func (tx *buildTransaction) StagePath(path string) string {
	if tx == nil {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	for i := range tx.entries {
		entry := &tx.entries[i]
		rel, relErr := filepath.Rel(entry.path, abs)
		if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			tx.prepareStage(entry)
			if rel == "." {
				return entry.stage
			}
			return filepath.Join(entry.stage, rel)
		}
	}
	return path
}

// prepareStage fills stage with the start-of-build snapshot the first time
// generation is pointed at it directly, so writes land on top of what existed.
func (tx *buildTransaction) prepareStage(entry *buildTransactionEntry) {
	if entry.staged {
		return
	}
	entry.staged = true
	if err := os.RemoveAll(entry.stage); err != nil {
		tx.recordStageErr(entry, err)
		return
	}
	if !entry.existed {
		return
	}
	if err := copyTransactionPath(entry.base, entry.stage); err != nil {
		tx.recordStageErr(entry, err)
	}
}

func (tx *buildTransaction) recordStageErr(entry *buildTransactionEntry, err error) {
	if tx.stageErr == nil {
		tx.stageErr = fmt.Errorf("%s: %w", entry.path, err)
	}
}

// mergeEntry publishes one owned path file by file; see buildTransactionEntry.
func (tx *buildTransaction) mergeEntry(entry *buildTransactionEntry) error {
	if !entry.staged {
		return nil
	}
	leaves := map[string]struct{}{}
	for _, root := range []string{entry.base, entry.stage, entry.path} {
		if err := collectTransactionLeaves(root, leaves); err != nil {
			return fmt.Errorf("list %s: %w", root, err)
		}
	}
	rels := make([]string, 0, len(leaves))
	for rel := range leaves {
		rels = append(rels, rel)
	}
	// A parent always sorts before its children, so a file that stands where the
	// generator now wants a directory is dealt with before anything goes inside.
	sort.Strings(rels)

	for _, rel := range rels {
		native := filepath.FromSlash(rel)
		base := filepath.Join(entry.base, native)
		stage := filepath.Join(entry.stage, native)
		current := filepath.Join(entry.path, native)

		generatorChanged, err := differentTransactionNodes(stage, base)
		if err != nil {
			return err
		}
		if !generatorChanged {
			continue
		}
		alreadyGenerated, err := sameTransactionNodes(current, stage)
		if err != nil {
			return err
		}
		if alreadyGenerated {
			continue
		}
		editedDuringBuild, err := differentTransactionNodes(current, base)
		if err != nil {
			return err
		}
		currentIsDir := isTransactionDir(current)
		if editedDuringBuild || currentIsDir {
			if err := tx.setAside(entry, rel, current); err != nil {
				return err
			}
		}
		if exists, err := transactionNodeExists(stage); err != nil {
			return err
		} else if exists {
			if err := tx.publish(entry, rel, stage, current); err != nil {
				return err
			}
		} else if err := tx.remove(entry, rel, current); err != nil {
			return err
		}
	}
	return pruneEmptyTransactionDirs(entry)
}

func (tx *buildTransaction) publish(entry *buildTransactionEntry, rel, stage, current string) error {
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		return fmt.Errorf("publish %s: %w", current, err)
	}
	backup := ""
	if exists, err := transactionNodeExists(current); err != nil {
		return err
	} else if exists {
		backup = filepath.Join(entry.backup, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
			return fmt.Errorf("back up %s: %w", current, err)
		}
		if err := os.Rename(current, backup); err != nil {
			return fmt.Errorf("move previous output %s: %w", current, err)
		}
		tx.journal = append(tx.journal, commitJournalOp{path: current, backup: backup})
	}
	if err := os.Rename(stage, current); err != nil {
		return fmt.Errorf("publish staged output %s: %w", current, err)
	}
	if backup != "" {
		tx.journal[len(tx.journal)-1].published = true
	} else {
		tx.journal = append(tx.journal, commitJournalOp{path: current, published: true})
	}
	return nil
}

func (tx *buildTransaction) remove(entry *buildTransactionEntry, rel, current string) error {
	exists, err := transactionNodeExists(current)
	if err != nil || !exists {
		return err
	}
	backup := filepath.Join(entry.backup, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		return fmt.Errorf("back up %s: %w", current, err)
	}
	if err := os.Rename(current, backup); err != nil {
		return fmt.Errorf("remove stale output %s: %w", current, err)
	}
	tx.journal = append(tx.journal, commitJournalOp{path: current, backup: backup})
	return nil
}

// setAside copies the project's version of a file (or a directory standing
// where a generated file goes) into the conflict directory before it is
// replaced. The copy is kept even if the commit is later rolled back.
func (tx *buildTransaction) setAside(entry *buildTransactionEntry, rel, current string) error {
	if exists, err := transactionNodeExists(current); err != nil || !exists {
		return err
	}
	root := tx.conflictRoot
	if strings.TrimSpace(tx.conflictDir) == "" {
		root = filepath.Dir(entry.path)
		tx.conflictDir = filepath.Join(root, ".ang", "conflicts", time.Now().UTC().Format("20060102T150405Z"))
	}
	display, err := filepath.Rel(root, current)
	if err != nil || display == ".." || strings.HasPrefix(display, ".."+string(filepath.Separator)) {
		display = strings.TrimPrefix(filepath.ToSlash(current), "/")
	}
	target := filepath.Join(tx.conflictDir, display)
	if err := copyTransactionPath(current, target); err != nil {
		return fmt.Errorf("save edited %s before regenerating it: %w", current, err)
	}
	tx.conflicts = append(tx.conflicts, filepath.ToSlash(display))
	return nil
}

func (tx *buildTransaction) undoJournal() error {
	var failures []string
	for i := len(tx.journal) - 1; i >= 0; i-- {
		op := tx.journal[i]
		if op.published {
			if err := os.RemoveAll(op.path); err != nil {
				failures = append(failures, err.Error())
				continue
			}
		}
		if op.backup == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(op.path), 0o755); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		if err := os.Rename(op.backup, op.path); err != nil {
			failures = append(failures, err.Error())
		}
	}
	tx.journal = nil
	if len(failures) != 0 {
		tx.keepWorkDirs = true
		for _, entry := range tx.entries {
			_ = writeBuildScratchOwner(entry.workDir, true)
		}
		return fmt.Errorf("restore committed outputs: %s", strings.Join(failures, "; "))
	}
	return nil
}

// pruneEmptyTransactionDirs removes directories that generation emptied: they
// existed at the start, are empty now, and generation does not have them.
// Directories created in the project during the build are never removed.
func pruneEmptyTransactionDirs(entry *buildTransactionEntry) error {
	info, err := os.Lstat(entry.path)
	if err != nil || !info.IsDir() {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var dirs []string
	if err := filepath.WalkDir(entry.path, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		rel, err := filepath.Rel(entry.path, dir)
		if err != nil {
			return err
		}
		if !isTransactionDir(filepath.Join(entry.base, rel)) || isTransactionDir(filepath.Join(entry.stage, rel)) {
			continue
		}
		children, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		if len(children) == 0 {
			if err := os.Remove(dir); err != nil {
				return err
			}
		}
	}
	return nil
}

// collectTransactionLeaves adds every non-directory entry under root, as a
// slash-separated path relative to root, to into. A root that is itself a file
// contributes the empty path.
func collectTransactionLeaves(root string, into map[string]struct{}) error {
	info, err := os.Lstat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		into[""] = struct{}{}
		return nil
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		into[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
}

func transactionNodeExists(path string) (bool, error) {
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isTransactionDir(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir()
}

func differentTransactionNodes(a, b string) (bool, error) {
	same, err := sameTransactionNodes(a, b)
	return !same, err
}

// sameTransactionNodes compares two leaves by presence, kind, symlink target
// and content. File modes are not compared.
func sameTransactionNodes(a, b string) (bool, error) {
	ai, aErr := os.Lstat(a)
	bi, bErr := os.Lstat(b)
	aMissing, bMissing := os.IsNotExist(aErr), os.IsNotExist(bErr)
	if aErr != nil && !aMissing {
		return false, aErr
	}
	if bErr != nil && !bMissing {
		return false, bErr
	}
	if aMissing || bMissing {
		return aMissing == bMissing, nil
	}
	if ai.Mode().Type() != bi.Mode().Type() {
		return false, nil
	}
	switch {
	case ai.Mode()&os.ModeSymlink != 0:
		aTarget, err := os.Readlink(a)
		if err != nil {
			return false, err
		}
		bTarget, err := os.Readlink(b)
		if err != nil {
			return false, err
		}
		return aTarget == bTarget, nil
	case ai.IsDir():
		// A directory standing at a leaf position; treated as unequal so the
		// caller never replaces a directory without saving it first.
		return false, nil
	}
	if ai.Size() != bi.Size() {
		return false, nil
	}
	return sameTransactionFileContent(a, b)
}

func sameTransactionFileContent(a, b string) (bool, error) {
	af, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer af.Close()
	bf, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer bf.Close()
	aBuf := make([]byte, 64<<10)
	bBuf := make([]byte, 64<<10)
	for {
		an, aErr := io.ReadFull(af, aBuf)
		bn, bErr := io.ReadFull(bf, bBuf)
		if an != bn || !bytes.Equal(aBuf[:an], bBuf[:bn]) {
			return false, nil
		}
		aDone := aErr == io.EOF || aErr == io.ErrUnexpectedEOF
		bDone := bErr == io.EOF || bErr == io.ErrUnexpectedEOF
		if aErr != nil && !aDone {
			return false, aErr
		}
		if bErr != nil && !bDone {
			return false, bErr
		}
		if aDone || bDone {
			return aDone == bDone, nil
		}
	}
}

// CreateWorkspace builds a copy-on-write project view. Top-level entries are
// symlinked until a generated path is materialized, so large source trees and
// caches are not copied merely to run generation and verification.
func (tx *buildTransaction) CreateWorkspace(projectRoot string) (string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	sweepAbandonedBuildScratch(filepath.Dir(root), workspaceScratchPrefix, time.Now(), &tx.swept)
	workDir, err := os.MkdirTemp(filepath.Dir(root), workspaceScratchPrefix+"*")
	if err != nil {
		return "", fmt.Errorf("create build workspace: %w", err)
	}
	_ = writeBuildScratchOwner(workDir, false)
	tx.workspaces = append(tx.workspaces, workDir)
	workspace := filepath.Join(workDir, "project")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return "", err
	}
	children, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, child := range children {
		if err := os.Symlink(filepath.Join(root, child.Name()), filepath.Join(workspace, child.Name())); err != nil {
			return "", err
		}
	}
	if err := rebaseWorkspaceLocalGoModReplaces(root, workspace); err != nil {
		return "", err
	}
	for _, entry := range tx.entries {
		rel, relErr := filepath.Rel(root, entry.path)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if err := materializeWorkspacePath(root, workspace, rel); err != nil {
			return "", fmt.Errorf("materialize staged path %s: %w", rel, err)
		}
	}
	return workspace, nil
}

// rebaseWorkspaceLocalGoModReplaces keeps a copy-on-write workspace usable
// when a project depends on a sibling module through a relative replace
// directive. The workspace is deliberately created outside of the project
// root, so the original relative path would otherwise point somewhere else.
//
// Only the workspace copy is changed; the project's go.mod remains untouched.
func rebaseWorkspaceLocalGoModReplaces(projectRoot, workspace string) error {
	sourcePath := filepath.Join(projectRoot, "go.mod")
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read project go.mod: %w", err)
	}

	parsed, err := modfile.Parse(sourcePath, data, nil)
	if err != nil {
		return fmt.Errorf("parse project go.mod: %w", err)
	}
	type replacement struct {
		oldPath    string
		oldVersion string
		newPath    string
		newVersion string
	}
	changes := make([]replacement, 0)
	for _, replace := range parsed.Replace {
		if !isRelativeLocalReplacePath(replace.New.Path) {
			continue
		}
		resolved := filepath.Clean(filepath.Join(projectRoot, replace.New.Path))
		info, statErr := os.Stat(resolved)
		if statErr != nil {
			return fmt.Errorf("resolve local go.mod replace %s => %s: %w", replace.Old.Path, replace.New.Path, statErr)
		}
		if !info.IsDir() {
			return fmt.Errorf("resolve local go.mod replace %s => %s: target %s is not a directory", replace.Old.Path, replace.New.Path, resolved)
		}
		changes = append(changes, replacement{
			oldPath: replace.Old.Path, oldVersion: replace.Old.Version,
			newPath: resolved, newVersion: replace.New.Version,
		})
	}
	if len(changes) == 0 {
		return nil
	}

	for _, change := range changes {
		if err := parsed.AddReplace(change.oldPath, change.oldVersion, change.newPath, change.newVersion); err != nil {
			return fmt.Errorf("rebase local go.mod replace %s: %w", change.oldPath, err)
		}
	}
	workspaceGoMod := filepath.Join(workspace, "go.mod")
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat project go.mod: %w", err)
	}
	if err := os.Remove(workspaceGoMod); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("materialize workspace go.mod: %w", err)
	}
	formatted, err := parsed.Format()
	if err != nil {
		return fmt.Errorf("format workspace go.mod: %w", err)
	}
	if err := os.WriteFile(workspaceGoMod, formatted, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write workspace go.mod: %w", err)
	}
	return nil
}

func isRelativeLocalReplacePath(path string) bool {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	return cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "."+string(filepath.Separator)) || strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}

// CaptureWorkspace copies only transaction-owned paths out of the overlay into
// their stages, ready to be merged into the project by Commit.
func (tx *buildTransaction) CaptureWorkspace(projectRoot, workspace string) error {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	for i := range tx.entries {
		entry := &tx.entries[i]
		rel, relErr := filepath.Rel(root, entry.path)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if err := os.RemoveAll(entry.stage); err != nil {
			return err
		}
		source := filepath.Join(workspace, rel)
		if _, statErr := os.Lstat(source); statErr == nil {
			if err := copyTransactionPath(source, entry.stage); err != nil {
				return fmt.Errorf("capture staged path %s: %w", rel, err)
			}
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		entry.staged = true
	}
	return nil
}

func materializeWorkspacePath(sourceRoot, workspace, rel string) error {
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	for index := range parts {
		partial := filepath.Join(parts[:index+1]...)
		destination := filepath.Join(workspace, partial)
		source := filepath.Join(sourceRoot, partial)
		if index == len(parts)-1 {
			if err := os.RemoveAll(destination); err != nil {
				return err
			}
			if _, err := os.Lstat(source); err == nil {
				return copyTransactionPath(source, destination)
			} else if os.IsNotExist(err) {
				return nil
			} else {
				return err
			}
		}
		info, err := os.Lstat(destination)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(destination); err != nil {
				return err
			}
			if err := os.Mkdir(destination, 0o755); err != nil {
				return err
			}
			children, readErr := os.ReadDir(source)
			if readErr != nil {
				return readErr
			}
			for _, child := range children {
				if err := os.Symlink(filepath.Join(source, child.Name()), filepath.Join(destination, child.Name())); err != nil {
					return err
				}
			}
		} else if os.IsNotExist(err) {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (tx *buildTransaction) cleanupWorkDirs() {
	if tx == nil || tx.keepWorkDirs {
		return
	}
	for _, entry := range tx.entries {
		_ = os.RemoveAll(entry.workDir)
	}
	for _, workspace := range tx.workspaces {
		_ = os.RemoveAll(workspace)
	}
}

func compactTransactionPaths(paths []string) []string {
	unique := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err == nil {
			unique[filepath.Clean(abs)] = struct{}{}
		}
	}
	out := make([]string, 0, len(unique))
	for path := range unique {
		out = append(out, path)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) < len(out[j])
		}
		return out[i] < out[j]
	})
	compacted := make([]string, 0, len(out))
	for _, candidate := range out {
		covered := false
		for _, parent := range compacted {
			rel, err := filepath.Rel(parent, candidate)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				covered = true
				break
			}
		}
		if !covered {
			compacted = append(compacted, candidate)
		}
	}
	return compacted
}

func copyTransactionPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	if !info.IsDir() {
		return copyTransactionFile(src, dst, info.Mode())
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyTransactionPath(path, target)
	})
}

func copyTransactionFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func generatedTransactionPaths(projectPath, mode string, backendDirs, frontendDirs []string) []string {
	projectAbs, _ := filepath.Abs(projectPath)
	paths := make([]string, 0, len(backendDirs)+len(frontendDirs)+12)
	for _, rel := range []string{"cmd/server", "internal", "api", "db", "deploy", "sdk", "scripts", "tests", "docs/ang", ".ang/cache", ".env.example", "ang-manifest.json", generatedFilesName, "atlas.hcl", "sqlc.yaml"} {
		paths = append(paths, filepath.Join(projectAbs, rel))
	}
	for _, backend := range backendDirs {
		backendAbs, _ := filepath.Abs(backend)
		if mode == "in_place" && backendAbs == projectAbs {
			continue
		}
		paths = append(paths, backendAbs)
	}
	paths = append(paths, frontendDirs...)
	return compactTransactionPaths(paths)
}
