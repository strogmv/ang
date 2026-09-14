package main

import (
	"os"
	"path/filepath"
	"strings"
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

// runWorkspaceBuild drives a transaction the way runBuild does: snapshot,
// workspace, "generation" in the workspace, then edits made to the real
// project while the build is still running, then capture and commit.
func runWorkspaceBuild(t *testing.T, root string, owned []string, generate func(workspace string), duringBuild func()) *buildTransaction {
	t.Helper()
	paths := make([]string, 0, len(owned))
	for _, rel := range owned {
		paths = append(paths, filepath.Join(root, rel))
	}
	tx, err := beginBuildTransaction(paths)
	if err != nil {
		t.Fatal(err)
	}
	tx.SetConflictDir(root, filepath.Join(root, ".ang", "conflicts", "test"))
	workspace, err := tx.CreateWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	generate(workspace)
	if duringBuild != nil {
		duringBuild()
	}
	if err := tx.CaptureWorkspace(root, workspace); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return tx
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertTestFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, data, want)
	}
}

func assertTestFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s should not exist (err=%v)", path, err)
	}
}

// The failure this merge exists for: a hand-written migration, an e2e test and
// an edit to a hand-maintained file, all made while a build ran, used to vanish
// when the snapshot of their directories was swapped back in.
func TestBuildTransactionCommitKeepsFilesWrittenDuringBuild(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal", "service", "user.gen.go"), "old generated")
	writeTestFile(t, filepath.Join(root, "internal", "service", "notes.go"), "hand v1")
	writeTestFile(t, filepath.Join(root, "db", "migrations", "001_init.sql"), "init")

	tx := runWorkspaceBuild(t, root, []string{"internal", "db", "tests"},
		func(ws string) {
			writeTestFile(t, filepath.Join(ws, "internal", "service", "user.gen.go"), "new generated")
			writeTestFile(t, filepath.Join(ws, "internal", "service", "order.gen.go"), "added by generator")
		},
		func() {
			writeTestFile(t, filepath.Join(root, "db", "migrations", "002_manual.sql"), "manual migration")
			writeTestFile(t, filepath.Join(root, "tests", "e2e", "manual.test.ts"), "manual test")
			writeTestFile(t, filepath.Join(root, "internal", "service", "notes.go"), "hand v2")
		})

	assertTestFile(t, filepath.Join(root, "internal", "service", "user.gen.go"), "new generated")
	assertTestFile(t, filepath.Join(root, "internal", "service", "order.gen.go"), "added by generator")
	assertTestFile(t, filepath.Join(root, "db", "migrations", "002_manual.sql"), "manual migration")
	assertTestFile(t, filepath.Join(root, "tests", "e2e", "manual.test.ts"), "manual test")
	assertTestFile(t, filepath.Join(root, "internal", "service", "notes.go"), "hand v2")
	assertTestFile(t, filepath.Join(root, "db", "migrations", "001_init.sql"), "init")
	if c := tx.Conflicts(); len(c) != 0 {
		t.Fatalf("no file was edited on both sides, got conflicts %v", c)
	}
}

func TestBuildTransactionCommitSavesEditOfRegeneratedFile(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "internal", "transport", "common.go")
	writeTestFile(t, generated, "generated v1")

	tx := runWorkspaceBuild(t, root, []string{"internal"},
		func(ws string) {
			writeTestFile(t, filepath.Join(ws, "internal", "transport", "common.go"), "generated v2")
		},
		func() {
			writeTestFile(t, generated, "my edit")
		})

	assertTestFile(t, generated, "generated v2")
	conflicts := tx.Conflicts()
	if len(conflicts) != 1 || conflicts[0] != "internal/transport/common.go" {
		t.Fatalf("conflicts = %v, want [internal/transport/common.go]", conflicts)
	}
	assertTestFile(t, filepath.Join(root, ".ang", "conflicts", "test", "internal", "transport", "common.go"), "my edit")
}

func TestBuildTransactionCommitAppliesOnlyGeneratorDeletions(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal", "stale", "old.gen.go"), "stale")
	writeTestFile(t, filepath.Join(root, "internal", "keep.gen.go"), "keep")
	writeTestFile(t, filepath.Join(root, "internal", "doomed.go"), "doomed")

	runWorkspaceBuild(t, root, []string{"internal"},
		func(ws string) {
			if err := os.RemoveAll(filepath.Join(ws, "internal", "stale")); err != nil {
				t.Fatal(err)
			}
		},
		func() {
			if err := os.Remove(filepath.Join(root, "internal", "doomed.go")); err != nil {
				t.Fatal(err)
			}
		})

	assertTestFileMissing(t, filepath.Join(root, "internal", "stale", "old.gen.go"))
	assertTestFileMissing(t, filepath.Join(root, "internal", "stale"))
	assertTestFile(t, filepath.Join(root, "internal", "keep.gen.go"), "keep")
	assertTestFileMissing(t, filepath.Join(root, "internal", "doomed.go"))
}

func TestBuildTransactionCommitKeepsDirectoryCreatedDuringBuild(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal", "a.gen.go"), "a")

	runWorkspaceBuild(t, root, []string{"internal"},
		func(ws string) {
			writeTestFile(t, filepath.Join(ws, "internal", "a.gen.go"), "a2")
		},
		func() {
			if err := os.MkdirAll(filepath.Join(root, "internal", "scratch"), 0o755); err != nil {
				t.Fatal(err)
			}
		})

	if info, err := os.Stat(filepath.Join(root, "internal", "scratch")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory created during the build was removed: %v", err)
	}
}

func TestBuildTransactionCommitRestoresProjectWhenPublishFails(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "api", "openapi.yaml")
	second := filepath.Join(root, "internal", "locked", "x.gen.go")
	writeTestFile(t, first, "api v1")
	writeTestFile(t, second, "x v1")

	tx, err := beginBuildTransaction([]string{filepath.Join(root, "api"), filepath.Join(root, "internal")})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := tx.CreateWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, "api", "openapi.yaml"), "api v2")
	writeTestFile(t, filepath.Join(workspace, "internal", "locked", "x.gen.go"), "x v2")
	if err := tx.CaptureWorkspace(root, workspace); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Dir(second)
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	if err := tx.Commit(); err == nil {
		t.Fatal("commit succeeded although internal/locked is read-only")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertTestFile(t, first, "api v1")
	assertTestFile(t, second, "x v1")
}

func TestBuildTransactionEntryNeverStagedIsLeftAlone(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "frontend-sdk")
	writeTestFile(t, filepath.Join(outside, "index.ts"), "sdk v1")

	tx, err := beginBuildTransaction([]string{outside})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := tx.CreateWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.CaptureWorkspace(root, workspace); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(outside, "index.ts"), "edited meanwhile")
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertTestFile(t, filepath.Join(outside, "index.ts"), "edited meanwhile")
}

// End to end through runBuild: files written into owned directories after
// generation finished but before publishing survive the build.
func TestRunBuildKeepsFilesWrittenDuringBuild(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "concurrent-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "concurrent-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/concurrent-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, _ := filepath.Abs("../..")
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/concurrent-app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")

	manual := map[string]string{
		filepath.Join("db", "migrations", "999_manual.sql"): "-- written during the build\n",
		filepath.Join("tests", "e2e", "manual.test.ts"):     "// written during the build\n",
		filepath.Join("internal", "manual_note.txt"):        "written during the build\n",
	}
	buildBeforeCommitHook = func(root string) {
		for rel, content := range manual {
			writeTestFile(t, filepath.Join(root, rel), content)
		}
	}
	t.Cleanup(func() { buildBeforeCommitHook = nil })

	if err := runBuild([]string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	for rel, content := range manual {
		assertTestFile(t, filepath.Join(projectDir, rel), content)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "internal", "domain", "user.go")); err != nil {
		t.Fatalf("generated output was not published: %v", err)
	}
}

func TestBuildTransactionWorkspaceRebasesLocalGoModReplace(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	dependency := filepath.Join(parent, "shared", "dependency")
	if err := os.MkdirAll(dependency, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	goMod := "module example.com/project\n\ngo 1.25\n\nreplace example.com/dependency => ../shared/dependency\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	tx, err := beginBuildTransaction([]string{filepath.Join(root, "internal")})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := tx.CreateWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	workspaceGoMod, err := os.ReadFile(filepath.Join(workspace, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(workspaceGoMod), "replace example.com/dependency => "+filepath.ToSlash(dependency); !strings.Contains(filepath.ToSlash(got), want) {
		t.Fatalf("workspace replace was not rebased:\n%s", got)
	}
	originalGoMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(originalGoMod); !strings.Contains(got, "=> ../shared/dependency") {
		t.Fatalf("source go.mod was changed:\n%s", got)
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

	if err := runBuild([]string{projectDir, "--mode=in_place", "--backend-dir=."}); err == nil {
		t.Fatal("runBuild succeeded despite a failed post-build Go verification")
	}

	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "preserve" {
		t.Fatalf("pre-build tree was not restored: data=%q err=%v", data, err)
	}
	generated := filepath.Join(projectDir, "internal", "domain", "user.go")
	if _, err := os.Stat(generated); !os.IsNotExist(err) {
		t.Fatalf("generated file survived failed build: %v", err)
	}
}
