package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A project whose SDK is generated straight into the application repository:
// the list carries those paths, and a file the build stops generating there is
// removed like any other, without the copy step that used to wipe the whole
// directory.
func TestRunBuildRemovesStaleSDKFilesInSiblingRepo(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "sibling-app")
	sdkDir := filepath.Join(root, "app", "src", "@sdk")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "sibling-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/sibling-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, _ := filepath.Abs("../..")
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/sibling-app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")
	build := []string{projectDir, "--mode=in_place", "--backend-dir=.", "--frontend-dir=" + sdkDir, "--skip-go-verify"}
	check := append(append([]string(nil), build...), "--check")

	if err := runBuild(build); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sdkDir, "index.ts")); err != nil {
		t.Fatalf("the SDK must be generated into the sibling repo: %v", err)
	}
	listed, hadList, err := readGeneratedFiles(projectDir)
	if err != nil || !hadList {
		t.Fatalf("read list: %v", err)
	}
	sdkRel := func(name string) string {
		rel, err := filepath.Rel(projectDir, filepath.Join(sdkDir, name))
		if err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(rel)
	}
	if _, ok := listed[sdkRel("index.ts")]; !ok {
		t.Fatalf("%s does not list the SDK outside the project (%d entries)", generatedFilesName, len(listed))
	}
	if err := runBuild(check); err != nil {
		t.Fatalf("check right after a build must pass: %v", err)
	}

	stale := filepath.Join(sdkDir, "ghost.ts")
	writeTestFile(t, stale, "export const ghost = 1;\n")
	handWritten := filepath.Join(sdkDir, "notes.txt")
	writeTestFile(t, handWritten, "kept\n")
	listPath := filepath.Join(projectDir, generatedFilesName)
	data, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, listPath, string(data)+sdkRel("ghost.ts")+"\n")

	if err := runBuild(check); err == nil {
		t.Fatal("check passed although a listed SDK file is no longer generated")
	}
	if err := runBuild(build); err != nil {
		t.Fatalf("second build: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the stale SDK file must be removed: %v", err)
	}
	assertTestFile(t, handWritten, "kept\n")
	if _, err := os.Stat(filepath.Join(sdkDir, "index.ts")); err != nil {
		t.Errorf("the generated SDK must stay: %v", err)
	}
}
