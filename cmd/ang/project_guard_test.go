package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestEnsureGeneratableProjectAcceptsProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cue", "api", "service.cue"), "package api\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/shop\n")

	if err := ensureGeneratableProject(dir, "cue"); err != nil {
		t.Fatalf("expected project to be generatable, got %v", err)
	}
	if err := ensureGeneratableProject(dir, ""); err != nil {
		t.Fatalf("expected default cue root to resolve, got %v", err)
	}
}

func TestEnsureGeneratableProjectRejectsGeneratorCheckout(t *testing.T) {
	dir := t.TempDir()
	// The generator checkout has a cue/ directory of its own, which is exactly
	// why the module check is needed: the CUE root alone would accept it.
	writeFile(t, filepath.Join(dir, "cue", "api", "service.cue"), "package api\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module "+generatorModulePath+"\n\ngo 1.24\n")

	err := ensureGeneratableProject(dir, "cue")
	if err == nil {
		t.Fatal("expected the generator checkout to be refused")
	}
	if !strings.Contains(err.Error(), "generator checkout") {
		t.Fatalf("expected a generator-checkout message, got %v", err)
	}
}

func TestEnsureGeneratableProjectRejectsGeneratorLayoutWithoutGoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cue", "api", "service.cue"), "package api\n")
	if err := os.MkdirAll(filepath.Join(dir, "compiler", "emitter"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "ang"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := ensureGeneratableProject(dir, "cue"); err == nil {
		t.Fatal("expected the generator layout to be refused")
	}
}

func TestEnsureGeneratableProjectRejectsDirectoryWithoutCueRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/unrelated\n")

	err := ensureGeneratableProject(dir, "cue")
	if err == nil {
		t.Fatal("expected a directory without a CUE root to be refused")
	}
	if !strings.Contains(err.Error(), "no CUE root") {
		t.Fatalf("expected a missing-CUE-root message, got %v", err)
	}
}

func TestEnsureGeneratableProjectRejectsMissingDirectory(t *testing.T) {
	if err := ensureGeneratableProject(filepath.Join(t.TempDir(), "absent"), "cue"); err == nil {
		t.Fatal("expected a missing directory to be refused")
	}
}

func TestParseOutputOptionsProjectFlag(t *testing.T) {
	opts, err := parseOutputOptions([]string{"--project", "/tmp/shop"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.ProjectDir != "/tmp/shop" {
		t.Fatalf("expected ProjectDir to be captured, got %q", opts.ProjectDir)
	}
	if opts, err := parseOutputOptions(nil); err != nil || opts.ProjectDir != "" {
		t.Fatalf("expected an empty ProjectDir by default, got %q (%v)", opts.ProjectDir, err)
	}
}
