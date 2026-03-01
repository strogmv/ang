package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeProjectName(t *testing.T) {
	if got := sanitizeProjectName("My Backend!"); got != "my-backend" {
		t.Fatalf("unexpected sanitized name: %s", got)
	}
}

func TestInitFromTemplateWritesFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "demo")
	err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    target,
		ProjectName:  "demo",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/demo",
	})
	if err != nil {
		t.Fatalf("initFromTemplate: %v", err)
	}

	required := []string{
		filepath.Join(target, "cue.mod", "module.cue"),
		filepath.Join(target, "cue", "project", "project.cue"),
		filepath.Join(target, "cue", "domain", "entities.cue"),
		filepath.Join(target, "cue", "api", "http.cue"),
		filepath.Join(target, "docker-compose.yml"),
		filepath.Join(target, ".env.example"),
		filepath.Join(target, "go.mod"),
		filepath.Join(target, "go.work"),
		filepath.Join(target, "Taskfile.yml"),
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file %s: %v", p, err)
		}
	}
}

func TestInitFromTemplateIncludesDevBootstrapArtifacts(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "demo")
	err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    target,
		ProjectName:  "demo",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/demo",
	})
	if err != nil {
		t.Fatalf("initFromTemplate: %v", err)
	}

	requiredScripts := []string{
		filepath.Join(target, "scripts", "dev-up.sh"),
		filepath.Join(target, "scripts", "dev-down.sh"),
		filepath.Join(target, "scripts", "dev-smoke.sh"),
		filepath.Join(target, "scripts", "dev-reset.sh"),
		filepath.Join(target, "scripts", "preflight.sh"),
	}
	for _, p := range requiredScripts {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected generated script %s: %v", p, err)
		}
	}

	goModData, err := os.ReadFile(filepath.Join(target, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	goWorkData, err := os.ReadFile(filepath.Join(target, "go.work"))
	if err != nil {
		t.Fatalf("read go.work: %v", err)
	}
	taskfileData, err := os.ReadFile(filepath.Join(target, "Taskfile.yml"))
	if err != nil {
		t.Fatalf("read Taskfile.yml: %v", err)
	}

	makefilePath := filepath.Join(target, "Makefile")
	makefileData, err := os.ReadFile(makefilePath)
	if err != nil {
		t.Fatalf("read %s: %v", makefilePath, err)
	}
	makefile := string(makefileData)
	for _, expected := range []string{"up:", "ang up", "doctor:", "scripts/preflight.sh", "smoke:", "dev-smoke.sh"} {
		if !strings.Contains(makefile, expected) {
			t.Fatalf("Makefile missing %q:\n%s", expected, makefile)
		}
	}
	if !strings.Contains(string(goModData), "module github.com/example/demo") {
		t.Fatalf("go.mod missing module path:\n%s", string(goModData))
	}
	if !strings.Contains(string(goWorkData), "use .") {
		t.Fatalf("go.work missing use directive:\n%s", string(goWorkData))
	}
	if !strings.Contains(string(taskfileData), "ang up") {
		t.Fatalf("Taskfile.yml missing up task:\n%s", string(taskfileData))
	}
}
