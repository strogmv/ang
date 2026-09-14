package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunBuildCheckDetectsDriftAndWritesNothing(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "check-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "check-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/check-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, _ := filepath.Abs("../..")
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/check-app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")

	build := []string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}
	check := append(append([]string(nil), build...), "--check")

	if err := runBuild(build); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	if err := runBuild(check); err != nil {
		t.Fatalf("check right after a build must pass: %v", err)
	}

	generated := filepath.Join(projectDir, "internal", "domain", "user.go")
	original, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	edited := string(original) + "\n// edited by hand\n"
	writeTestFile(t, generated, edited)

	if err := runBuild(check); err == nil {
		t.Fatal("check passed although a generated file was edited by hand")
	}
	assertTestFile(t, generated, edited)
}

func TestDryRunDifferencesReportsOnlyCreatesAndUpdates(t *testing.T) {
	root := t.TempDir()
	man := dryRunManifest{Targets: []dryRunTargetManifest{{Changes: []dryRunFileChange{
		{Path: filepath.ToSlash(filepath.Join(root, "internal", "b.go")), Action: "update"},
		{Path: filepath.ToSlash(filepath.Join(root, "internal", "same.go")), Action: "unchanged"},
		{Path: filepath.ToSlash(filepath.Join(root, "api", "a.yaml")), Action: "create"},
	}}}}
	got := dryRunDifferences(man, root)
	if len(got) != 2 || got[0].Path != "api/a.yaml" || got[0].Action != "create" || got[1].Path != "internal/b.go" || got[1].Action != "update" {
		t.Fatalf("differences = %+v", got)
	}
}
