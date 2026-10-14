package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The files a build writes next to the generated code and a project commits
// (ang-generated.txt, ang-manifest.json, docs/ang/conventions.md) depend on
// the input alone: the same CUE built twice, or built from another directory,
// gives the same bytes, and a build that changes nothing does not rewrite them.
func TestBuildSideFilesAreDeterministic(t *testing.T) {
	sideFiles := []string{generatedFilesName, "ang-manifest.json", filepath.Join("docs", "ang", "conventions.md")}
	root := t.TempDir()
	one := writeSideFileProject(t, filepath.Join(root, "one"))
	two := writeSideFileProject(t, filepath.Join(root, "elsewhere", "two"))
	build := func(project string) {
		t.Helper()
		if err := runBuild([]string{project, "--skip-go-verify"}); err != nil {
			t.Fatalf("build %s: %v", project, err)
		}
	}
	read := func(project string) map[string][]byte {
		t.Helper()
		out := map[string][]byte{}
		for _, name := range sideFiles {
			data, err := os.ReadFile(filepath.Join(project, name))
			if err != nil {
				t.Fatalf("the build must write %s: %v", name, err)
			}
			out[name] = data
		}
		return out
	}

	build(one)
	first := read(one)
	if !strings.Contains(string(first[generatedFilesName]), "../front/src/@sdk/") {
		t.Fatalf("%s does not list the SDK written next to the project:\n%s", generatedFilesName, first[generatedFilesName])
	}
	for name, data := range first {
		if bytes.Contains(data, []byte(root)) {
			t.Errorf("%s contains the absolute path of the project", name)
		}
	}

	// Rebuild with old modification times: identical files stay untouched.
	old := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, name := range sideFiles {
		if err := os.Chtimes(filepath.Join(one, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	build(one)
	again := read(one)
	for _, name := range sideFiles {
		if !bytes.Equal(first[name], again[name]) {
			t.Errorf("%s changed on a rebuild of the same input", name)
		}
		info, err := os.Stat(filepath.Join(one, name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(old) {
			t.Errorf("%s was rewritten although its content did not change", name)
		}
	}

	build(two)
	elsewhere := read(two)
	for _, name := range sideFiles {
		if !bytes.Equal(first[name], elsewhere[name]) {
			t.Errorf("%s differs between two directories holding the same project", name)
		}
	}
}

// writeSideFileProject scaffolds the saas template at dir with its SDK going
// to ../front/src/@sdk, the layout of a backend next to its frontend app.
func writeSideFileProject(t *testing.T, dir string) string {
	t.Helper()
	project := filepath.Join(dir, "app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    project,
		ProjectName:  "app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(project, "go.mod"), "module github.com/example/app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")
	writeTestFile(t, filepath.Join(project, "cue", "project", "project.cue"), `package project

state: {
	target: {
		lang:         "go"
		framework:    "chi"
		db:           "postgres"
		frontend_dir: "../front/src/@sdk"
	}
}
`)
	if err := os.MkdirAll(filepath.Join(dir, "front", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	return project
}
