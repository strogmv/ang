package main

import (
	"os"
	"path/filepath"
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
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file %s: %v", p, err)
		}
	}
}
