package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// A dry run used to say that a file would be updated, not how. Reviewing a CUE
// change therefore meant writing the generated result into the project first.
// With --diff (or --diff-out) the dry run carries a unified diff for every
// generated file it would create or change.

const dryRunDiffContext = 3

// unifiedFileDiff renders before → after for one file. A file that does not
// exist yet is diffed against /dev/null; content with a NUL byte in its first
// 8 KB is reported as binary rather than dumped.
func unifiedFileDiff(label string, before, after []byte, existed bool) string {
	from := "a/" + label
	if !existed {
		from = "/dev/null"
		before = nil
	}
	if looksBinary(before) || looksBinary(after) {
		return fmt.Sprintf("Binary files %s and b/%s differ\n", from, label)
	}
	var beforeLines []string
	if len(before) > 0 {
		beforeLines = difflib.SplitLines(string(before))
	}
	var afterLines []string
	if len(after) > 0 {
		afterLines = difflib.SplitLines(string(after))
	}
	text, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        beforeLines,
		B:        afterLines,
		FromFile: from,
		ToFile:   "b/" + label,
		Context:  dryRunDiffContext,
	})
	if err != nil {
		return fmt.Sprintf("--- %s\n+++ b/%s\n(diff unavailable: %v)\n", from, label, err)
	}
	return text
}

func looksBinary(content []byte) bool {
	if len(content) > 8000 {
		content = content[:8000]
	}
	return bytes.IndexByte(content, 0) >= 0
}

// dryRunDiffLabel names dest relative to root; a path outside root keeps its
// absolute form without the leading slash, so it can never escape a
// --diff-out directory.
func dryRunDiffLabel(root, dest string) string {
	rootAbs, rootErr := filepath.Abs(root)
	destAbs, destErr := filepath.Abs(dest)
	if rootErr == nil && destErr == nil {
		if rel, err := filepath.Rel(rootAbs, destAbs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(destAbs)), "/")
}

// dryRunDiffs returns the changes that carry a diff, ordered by label.
func dryRunDiffs(man dryRunManifest) []dryRunFileChange {
	var out []dryRunFileChange
	for _, target := range man.Targets {
		for _, change := range target.Changes {
			if change.Diff != "" {
				out = append(out, change)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

func printDryRunDiffs(out *os.File, diffs []dryRunFileChange) {
	for _, change := range diffs {
		fmt.Fprint(out, change.Diff)
		if !strings.HasSuffix(change.Diff, "\n") {
			fmt.Fprintln(out)
		}
	}
}

// writeDryRunDiffs writes each diff to <dir>/<label>.patch.
func writeDryRunDiffs(dir string, diffs []dryRunFileChange) error {
	for _, change := range diffs {
		target := filepath.Join(dir, filepath.FromSlash(change.Label)+".patch")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(change.Diff), 0o644); err != nil {
			return err
		}
	}
	return nil
}
