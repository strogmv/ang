package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildDryRunChanges(t *testing.T) {
	tmp := t.TempDir()
	genRoot := filepath.Join(tmp, "gen")
	dstRoot := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(genRoot, 0o755); err != nil {
		t.Fatalf("mkdir gen: %v", err)
	}
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(genRoot, "a"), 0o755); err != nil {
		t.Fatalf("mkdir gen/a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(genRoot, "a", "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write gen new: %v", err)
	}
	if err := os.WriteFile(filepath.Join(genRoot, "a", "same.txt"), []byte("same"), 0o644); err != nil {
		t.Fatalf("write gen same: %v", err)
	}
	if err := os.WriteFile(filepath.Join(genRoot, "a", "upd.txt"), []byte("new-value"), 0o644); err != nil {
		t.Fatalf("write gen upd: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(dstRoot, "a"), 0o755); err != nil {
		t.Fatalf("mkdir dst/a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstRoot, "a", "same.txt"), []byte("same"), 0o644); err != nil {
		t.Fatalf("write dst same: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstRoot, "a", "upd.txt"), []byte("old-value"), 0o644); err != nil {
		t.Fatalf("write dst upd: %v", err)
	}

	changes, err := buildDryRunChanges(genRoot, dstRoot)
	if err != nil {
		t.Fatalf("buildDryRunChanges: %v", err)
	}
	if len(changes) != 3 {
		t.Fatalf("len(changes)=%d, want 3", len(changes))
	}

	got := map[string]string{}
	for _, c := range changes {
		got[filepath.Base(c.Path)] = c.Action
	}
	if got["new.txt"] != "create" {
		t.Fatalf("new.txt action=%q, want create", got["new.txt"])
	}
	if got["upd.txt"] != "update" {
		t.Fatalf("upd.txt action=%q, want update", got["upd.txt"])
	}
	if got["same.txt"] != "unchanged" {
		t.Fatalf("same.txt action=%q, want unchanged", got["same.txt"])
	}
}
