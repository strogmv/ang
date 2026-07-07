package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateHashIsStableAndTracksPathsAndContent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(root, "a.cue")
	second := filepath.Join(root, "nested", "b.cue")
	if err := os.WriteFile(first, []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}

	h1, err := calculateHash([]string{root})
	if err != nil {
		t.Fatalf("first hash: %v", err)
	}
	h2, err := calculateHash([]string{root})
	if err != nil {
		t.Fatalf("second hash: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("hash is not stable: %s != %s", h1, h2)
	}

	if err := os.WriteFile(second, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	h3, err := calculateHash([]string{root})
	if err != nil {
		t.Fatalf("changed hash: %v", err)
	}
	if h3 == h1 {
		t.Fatal("content change did not invalidate input hash")
	}
}

func TestCalculateHashRejectsMissingAndEmptyDirectories(t *testing.T) {
	if _, err := calculateHash([]string{filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected missing directory error")
	}
	if _, err := calculateHash([]string{t.TempDir()}); err == nil {
		t.Fatal("expected empty directory error")
	}
}

func TestCalculateHashDoesNotDependOnAbsoluteRoot(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	for _, root := range roots {
		if err := os.WriteFile(filepath.Join(root, "same.cue"), []byte("same"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	first, err := calculateHash([]string{roots[0]})
	if err != nil {
		t.Fatal(err)
	}
	second, err := calculateHash([]string{roots[1]})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("hash depends on absolute root: %s != %s", first, second)
	}
}
