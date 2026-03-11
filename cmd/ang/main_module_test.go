package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadGoModuleAt_FromGoMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/from-gomod\n\ngo 1.24.0\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if got := readGoModuleAt(root); got != "example.com/from-gomod" {
		t.Fatalf("readGoModuleAt() = %q, want %q", got, "example.com/from-gomod")
	}
}

func TestReadGoModuleAt_FromAngYAML(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ang.yaml"), []byte("go:\n  module: \"example.com/from-ang-yaml\"\n"), 0644); err != nil {
		t.Fatalf("write ang.yaml: %v", err)
	}
	if got := readGoModuleAt(root); got != "example.com/from-ang-yaml" {
		t.Fatalf("readGoModuleAt() = %q, want %q", got, "example.com/from-ang-yaml")
	}
}
