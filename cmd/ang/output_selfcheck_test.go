package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoListDirOutput_LastPathLineWins(t *testing.T) {
	t.Parallel()

	out := []byte("go: downloading x/y v1.2.3\n/home/strog/work/sendbox/cmd/server\n")
	got, err := parseGoListDirOutput(out)
	if err != nil {
		t.Fatalf("parseGoListDirOutput: %v", err)
	}
	want := "/home/strog/work/sendbox/cmd/server"
	if got != want {
		t.Fatalf("unexpected parsed dir: got=%q want=%q", got, want)
	}
}

func TestParseGoListDirOutput_EmptyFails(t *testing.T) {
	t.Parallel()

	if _, err := parseGoListDirOutput([]byte("\n \n")); err == nil {
		t.Fatalf("expected error for empty output")
	}
}

func TestSameResolvedPathAcceptsSymlinkAlias(t *testing.T) {
	realDir := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if !sameResolvedPath(realDir, alias) {
		t.Fatalf("expected symlink aliases to compare equal: %s vs %s", realDir, alias)
	}
}
