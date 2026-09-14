package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnifiedFileDiffUpdate(t *testing.T) {
	diff := unifiedFileDiff("internal/a.go", []byte("package a\n\nconst X = 1\n"), []byte("package a\n\nconst X = 2\n"), true)
	for _, want := range []string{"--- a/internal/a.go\n", "+++ b/internal/a.go\n", "\n-const X = 1\n", "\n+const X = 2\n"} {
		if !strings.Contains(diff, want) {
			t.Fatalf("diff lacks %q:\n%s", want, diff)
		}
	}
}

func TestUnifiedFileDiffCreateStartsFromDevNull(t *testing.T) {
	diff := unifiedFileDiff("api/new.yaml", nil, []byte("openapi: 3.0.0\n"), false)
	if !strings.Contains(diff, "--- /dev/null\n") || !strings.Contains(diff, "+++ b/api/new.yaml\n") || !strings.Contains(diff, "\n+openapi: 3.0.0\n") {
		t.Fatalf("create diff:\n%s", diff)
	}
	for _, line := range strings.Split(diff, "\n") {
		if line == "-" {
			t.Fatalf("a new file must not remove an empty line:\n%s", diff)
		}
	}
}

func TestUnifiedFileDiffBinary(t *testing.T) {
	diff := unifiedFileDiff("assets/logo.png", []byte{0x89, 0, 1}, []byte{0x89, 0, 2}, true)
	if !strings.HasPrefix(diff, "Binary files a/assets/logo.png and b/assets/logo.png differ") {
		t.Fatalf("binary diff = %q", diff)
	}
}

func TestDryRunDiffLabelNeverEscapes(t *testing.T) {
	root := t.TempDir()
	if got := dryRunDiffLabel(root, filepath.Join(root, "sdk", "index.ts")); got != "sdk/index.ts" {
		t.Fatalf("label inside the project = %q", got)
	}
	outside := filepath.Join(filepath.Dir(root), "elsewhere", "x.ts")
	got := dryRunDiffLabel(root, outside)
	if strings.HasPrefix(got, "/") || strings.Contains(got, "..") {
		t.Fatalf("label outside the project may escape --diff-out: %q", got)
	}
}

func TestRunBuildDryRunDiffWritesPatchesOnly(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "diff-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "diff-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/diff-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, _ := filepath.Abs("../..")
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/diff-app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")

	build := []string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}
	if err := runBuild(build); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	generated := filepath.Join(projectDir, "internal", "domain", "user.go")
	original, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	edited := string(original) + "// edited by hand\n"
	writeTestFile(t, generated, edited)

	patches := filepath.Join(t.TempDir(), "patches")
	if err := runBuild(append(append([]string(nil), build...), "--dry-run", "--diff-out", patches)); err != nil {
		t.Fatalf("dry run with --diff-out: %v", err)
	}

	patch, err := os.ReadFile(filepath.Join(patches, "internal", "domain", "user.go.patch"))
	if err != nil {
		t.Fatalf("no patch for the edited file: %v", err)
	}
	if !strings.Contains(string(patch), "\n-// edited by hand\n") || !strings.Contains(string(patch), "--- a/internal/domain/user.go\n") {
		t.Fatalf("patch does not undo the hand edit:\n%s", patch)
	}
	assertTestFile(t, generated, edited)

	entries := 0
	_ = filepath.WalkDir(patches, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			entries++
		}
		return nil
	})
	if entries != 1 {
		t.Fatalf("wrote %d patches, want exactly the one for the edited file", entries)
	}
}

func TestDiffFlagsRequireDryRunOrCheck(t *testing.T) {
	if _, err := parseOutputOptions([]string{"--diff"}); err == nil {
		t.Fatal("--diff without --dry-run or --check must be rejected")
	}
	opts, err := parseOutputOptions([]string{"--check", "--diff-out", "/tmp/x"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.DryRun || !opts.DryRunDiff || opts.DryRunDiffOut != "/tmp/x" {
		t.Fatalf("opts = %+v", opts)
	}
}
