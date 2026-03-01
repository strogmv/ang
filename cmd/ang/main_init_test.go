package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitLegacyScaffoldCreatesMinimalFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := initLegacyScaffold(root, "github.com/acme/minapp", "go", "postgres"); err != nil {
		t.Fatalf("initLegacyScaffold: %v", err)
	}

	goModPath := filepath.Join(root, "go.mod")
	goWorkPath := filepath.Join(root, "go.work")
	taskfilePath := filepath.Join(root, "Taskfile.yml")
	required := []string{
		filepath.Join(root, "cue.mod", "module.cue"),
		filepath.Join(root, "cue", "project", "project.cue"),
		filepath.Join(root, "cue", "domain", "entities.cue"),
		filepath.Join(root, "cue", "architecture", "services.cue"),
		filepath.Join(root, "cue", "api", "http.cue"),
		filepath.Join(root, "cue", "api", "operations.cue"),
		filepath.Join(root, "cue", "repo", "repositories.cue"),
		filepath.Join(root, "cue", "policies", "rbac.cue"),
		goModPath,
		goWorkPath,
		taskfilePath,
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file %s: %v", p, err)
		}
	}

	moduleData, err := os.ReadFile(filepath.Join(root, "cue.mod", "module.cue"))
	if err != nil {
		t.Fatalf("read module.cue: %v", err)
	}
	if !strings.Contains(string(moduleData), `module: "github.com/acme/minapp"`) {
		t.Fatalf("module.cue missing module path:\n%s", string(moduleData))
	}

	projectData, err := os.ReadFile(filepath.Join(root, "cue", "project", "project.cue"))
	if err != nil {
		t.Fatalf("read project.cue: %v", err)
	}
	if !strings.Contains(string(projectData), `lang:      "go"`) {
		t.Fatalf("project.cue missing lang:\n%s", string(projectData))
	}

	goModData, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(goModData), "module github.com/acme/minapp") {
		t.Fatalf("go.mod missing module path:\n%s", string(goModData))
	}

	goWorkData, err := os.ReadFile(goWorkPath)
	if err != nil {
		t.Fatalf("read go.work: %v", err)
	}
	if !strings.Contains(string(goWorkData), "use .") {
		t.Fatalf("go.work missing use directive:\n%s", string(goWorkData))
	}

	taskfileData, err := os.ReadFile(taskfilePath)
	if err != nil {
		t.Fatalf("read Taskfile.yml: %v", err)
	}
	if !strings.Contains(string(taskfileData), "ang up") {
		t.Fatalf("Taskfile.yml missing up task:\n%s", string(taskfileData))
	}
}

func TestInitLegacyScaffoldDoesNotOverrideExistingFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "cue", "api", "operations.cue")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const custom = "package api\n\nCustom: true\n"
	if err := os.WriteFile(target, []byte(custom), 0644); err != nil {
		t.Fatalf("write custom file: %v", err)
	}

	if err := initLegacyScaffold(root, "github.com/acme/minapp", "go", "postgres"); err != nil {
		t.Fatalf("initLegacyScaffold: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read operations.cue: %v", err)
	}
	if string(got) != custom {
		t.Fatalf("expected existing file to remain unchanged, got:\n%s", string(got))
	}
}
