package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type buildTransactionEntry struct {
	path      string
	backup    string
	stage     string
	workDir   string
	existed   bool
	committed bool
}

type buildTransaction struct {
	root       string
	entries    []buildTransactionEntry
	workspaces []string
	done       bool
}

func beginBuildTransaction(paths []string) (*buildTransaction, error) {
	tx := &buildTransaction{}
	cleaned := compactTransactionPaths(paths)
	for _, path := range cleaned {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tx.cleanupWorkDirs()
			return nil, fmt.Errorf("create transaction parent for %s: %w", path, err)
		}
		workDir, err := os.MkdirTemp(filepath.Dir(path), ".ang-build-transaction-*")
		if err != nil {
			tx.cleanupWorkDirs()
			return nil, fmt.Errorf("create build transaction for %s: %w", path, err)
		}
		entry := buildTransactionEntry{path: path, backup: filepath.Join(workDir, "backup"), stage: filepath.Join(workDir, "stage"), workDir: workDir}
		if _, err := os.Lstat(path); err == nil {
			entry.existed = true
			if err := copyTransactionPath(path, entry.stage); err != nil {
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

func (tx *buildTransaction) Commit() error {
	if tx == nil || tx.done {
		return nil
	}
	for i := range tx.entries {
		entry := &tx.entries[i]
		if entry.existed {
			if err := os.Rename(entry.path, entry.backup); err != nil {
				_ = tx.restoreCommitted(i - 1)
				return fmt.Errorf("move previous output %s: %w", entry.path, err)
			}
		}
		if _, err := os.Lstat(entry.stage); err == nil {
			if err := os.Rename(entry.stage, entry.path); err != nil {
				if entry.existed {
					_ = os.Rename(entry.backup, entry.path)
				}
				_ = tx.restoreCommitted(i - 1)
				return fmt.Errorf("publish staged output %s: %w", entry.path, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect staged output %s: %w", entry.stage, err)
		}
		entry.committed = true
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
	for _, entry := range tx.entries {
		rel, relErr := filepath.Rel(entry.path, abs)
		if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if rel == "." {
				return entry.stage
			}
			return filepath.Join(entry.stage, rel)
		}
	}
	return path
}

// CreateWorkspace builds a copy-on-write project view. Top-level entries are
// symlinked until a generated path is materialized, so large source trees and
// caches are not copied merely to run generation and verification.
func (tx *buildTransaction) CreateWorkspace(projectRoot string) (string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	workDir, err := os.MkdirTemp(filepath.Dir(root), ".ang-build-workspace-*")
	if err != nil {
		return "", fmt.Errorf("create build workspace: %w", err)
	}
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

// CaptureWorkspace copies only transaction-owned paths out of the overlay.
// The resulting per-path staging entries can then be published with renames.
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

func (tx *buildTransaction) restoreCommitted(last int) error {
	var failures []string
	for i := last; i >= 0; i-- {
		entry := &tx.entries[i]
		if !entry.committed {
			continue
		}
		if err := os.RemoveAll(entry.path); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		if entry.existed {
			if err := os.Rename(entry.backup, entry.path); err != nil {
				failures = append(failures, err.Error())
			}
		}
		entry.committed = false
	}
	if len(failures) != 0 {
		return fmt.Errorf("restore committed outputs: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (tx *buildTransaction) cleanupWorkDirs() {
	if tx == nil {
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
	for _, rel := range []string{"cmd/server", "internal", "api", "db", "deploy", "sdk", "scripts", "tests", ".ang/cache", ".env.example", "ang-manifest.json", "atlas.hcl", "sqlc.yaml"} {
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
