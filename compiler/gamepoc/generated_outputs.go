package gamepoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Track actual writes, rather than guessing ownership from Git or suffixes.
// The existing phase clock and this inventory belong to one invocation.
var outputWrites = struct {
	sync.Mutex
	paths map[string]bool
}{}

var generationInventoryMu sync.Mutex

func beginGeneratedOutputs() func() {
	generationInventoryMu.Lock()
	outputWrites.Lock()
	outputWrites.paths = map[string]bool{}
	outputWrites.Unlock()
	return func() {
		outputWrites.Lock()
		outputWrites.paths = nil
		outputWrites.Unlock()
		generationInventoryMu.Unlock()
	}
}

func writeGeneratedFile(path string, data []byte, mode os.FileMode) error {
	if err := os.WriteFile(path, data, mode); err != nil {
		return err
	}
	outputWrites.Lock()
	defer outputWrites.Unlock()
	if outputWrites.paths != nil {
		outputWrites.paths[filepath.Clean(path)] = true
	}
	return nil
}

type generatedOutput struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Tracked bool   `json:"tracked"`
}

func writeGeneratedOutputs(root string, paths []string) error {
	cmd := exec.Command("git", "-C", root, "ls-files", "-z")
	raw, err := cmd.Output()
	if err != nil {
		// Build snapshots deliberately have no index: none of their outputs are tracked.
		if _, statErr := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(statErr) {
			return fmt.Errorf("generated manifest git index: %w", err)
		}
	}
	tracked := map[string]bool{}
	for _, name := range strings.Split(string(raw), "\x00") {
		tracked[name] = true
	}
	rows := make([]generatedOutput, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return fmt.Errorf("generated output escapes project: %s", path)
		}
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			continue
		}
		seen[rel] = true
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		rows = append(rows, generatedOutput{rel, hex.EncodeToString(hash[:]), tracked[rel]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	data, err := json.MarshalIndent(struct {
		Version int               `json:"version"`
		Files   []generatedOutput `json:"files"`
	}{1, rows}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(root, ".local", "ang")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	temporary := filepath.Join(dir, "generated-outputs.json.tmp")
	if err := os.WriteFile(temporary, append(data, '\n'), 0644); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(dir, "generated-outputs.json"))
}

func finishGeneratedOutputs(project string) error {
	outputWrites.Lock()
	paths := make([]string, 0, len(outputWrites.paths))
	for path := range outputWrites.paths {
		paths = append(paths, path)
	}
	outputWrites.Unlock()
	return writeGeneratedOutputs(filepath.Dir(project), paths)
}
