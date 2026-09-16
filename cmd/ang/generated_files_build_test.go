package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A build removes what the project stopped generating, and only that: a file
// ang-generated.txt never listed is left alone. --check reports the stale file
// before the build removes it.
func TestRunBuildRemovesFilesItNoLongerGenerates(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "stale-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "stale-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/stale-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, _ := filepath.Abs("../..")
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/stale-app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")
	build := []string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify"}
	check := append(append([]string(nil), build...), "--check")

	if err := runBuild(build); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	listed, hadList, err := readGeneratedFiles(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if !hadList {
		t.Fatalf("the build must write %s", generatedFilesName)
	}
	if _, ok := listed["internal/domain/user.go"]; !ok {
		t.Fatalf("%s does not list generated code (%d entries)", generatedFilesName, len(listed))
	}
	if err := runBuild(check); err != nil {
		t.Fatalf("check right after a build must pass: %v", err)
	}

	// A file an earlier build generated and this one does not, next to a
	// hand-written file that was never generated.
	stale := "internal/domain/ghost.go"
	writeTestFile(t, filepath.Join(projectDir, filepath.FromSlash(stale)), "package domain\n")
	handWritten := "internal/domain/notes.txt"
	writeTestFile(t, filepath.Join(projectDir, filepath.FromSlash(handWritten)), "kept\n")
	listPath := filepath.Join(projectDir, generatedFilesName)
	data, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, listPath, string(data)+stale+"\n")

	if err := runBuild(check); err == nil {
		t.Fatal("check passed although a listed file is no longer generated")
	}
	if err := runBuild(build); err != nil {
		t.Fatalf("second build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(stale))); !os.IsNotExist(err) {
		t.Errorf("%s must be removed: %v", stale, err)
	}
	assertTestFile(t, filepath.Join(projectDir, filepath.FromSlash(handWritten)), "kept\n")
	if _, err := os.Stat(filepath.Join(projectDir, "internal", "domain", "user.go")); err != nil {
		t.Errorf("generated code must stay: %v", err)
	}
	listed, _, err = readGeneratedFiles(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := listed[stale]; ok {
		t.Error("a removed file must be gone from the list")
	}
	if err := runBuild(check); err != nil {
		t.Fatalf("check after the build must pass: %v", err)
	}
}
