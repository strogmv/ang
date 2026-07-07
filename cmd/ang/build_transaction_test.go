package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildTransactionRollsBackUpdatesCreatesAndDeletes(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "generated")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(existing, "original.go")
	if err := os.WriteFile(original, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	createdRoot := filepath.Join(root, "new-sdk")

	tx, err := beginBuildTransaction([]string{existing, createdRoot})
	if err != nil {
		t.Fatal(err)
	}
	stagedExisting := tx.StagePath(existing)
	stagedCreated := tx.StagePath(createdRoot)
	if err := os.WriteFile(filepath.Join(stagedExisting, "original.go"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedExisting, "new.go"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stagedCreated, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedCreated, "index.ts"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(original)
	if err != nil || string(data) != "original" {
		t.Fatalf("original file not restored: data=%q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(existing, "new.go")); !os.IsNotExist(err) {
		t.Fatalf("created file survived rollback: %v", err)
	}
	if _, err := os.Stat(createdRoot); !os.IsNotExist(err) {
		t.Fatalf("created root survived rollback: %v", err)
	}
}

func TestBuildTransactionCommitKeepsChanges(t *testing.T) {
	root := filepath.Join(t.TempDir(), "generated")
	tx, err := beginBuildTransaction([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	stagedRoot := tx.StagePath(root)
	if err := os.MkdirAll(stagedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedRoot, "kept.go"), []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "kept.go")); err != nil {
		t.Fatalf("committed file missing: %v", err)
	}
}

func TestBuildTransactionWorkspacePublishesOnlyTrackedPaths(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "internal")
	if err := os.MkdirAll(generated, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generated, "old.go"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "cue")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "intent.cue"), []byte("intent"), 0o644); err != nil {
		t.Fatal(err)
	}

	tx, err := beginBuildTransaction([]string{generated})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := tx.CreateWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "cue", "intent.cue")); err != nil || string(data) != "intent" {
		t.Fatalf("source symlink unavailable: data=%q err=%v", data, err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "internal", "new.go"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(generated, "new.go")); !os.IsNotExist(err) {
		t.Fatalf("staged output leaked before commit: %v", err)
	}
	if err := tx.CaptureWorkspace(root, workspace); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(generated, "new.go")); err != nil || string(data) != "new" {
		t.Fatalf("staged output not published: data=%q err=%v", data, err)
	}
}

func TestRunBuildRollsBackGeneratedTreeWhenPostVerifyFails(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "rollback-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "rollback-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/rollback-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	// Deliberately incomplete module dependencies make post-build go verify fail.
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module github.com/example/rollback-app\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(projectDir, "internal", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}

	runBuild([]string{projectDir, "--mode=in_place", "--backend-dir=."})

	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "preserve" {
		t.Fatalf("pre-build tree was not restored: data=%q err=%v", data, err)
	}
	generated := filepath.Join(projectDir, "internal", "domain", "user.go")
	if _, err := os.Stat(generated); !os.IsNotExist(err) {
		t.Fatalf("generated file survived failed build: %v", err)
	}
}
