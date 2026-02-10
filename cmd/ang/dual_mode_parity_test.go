package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDualModeParity_BasicGeneratedDomain(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "parity-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "parity-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/parity-app",
		Force:        true,
	}); err != nil {
		t.Fatalf("initFromTemplate: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(projectDir, "go.mod"),
		[]byte("module github.com/example/parity-app\n\ngo 1.25\n"),
		0o644,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// In-place build writes generated code into project root.
	runBuild([]string{projectDir, "--mode=in_place", "--backend-dir=."})
	inPlaceFile := filepath.Join(projectDir, "internal", "domain", "user.go")
	inPlaceData, err := os.ReadFile(inPlaceFile)
	if err != nil {
		t.Fatalf("read in-place domain file: %v", err)
	}

	// Release build writes generated code into dist/release.
	runBuild([]string{projectDir, "--mode=release"})
	releaseFile := filepath.Join(projectDir, "dist", "release", "go-service", "internal", "domain", "user.go")
	releaseData, err := os.ReadFile(releaseFile)
	if err != nil {
		t.Fatalf("read release domain file: %v", err)
	}

	// Behavior parity baseline: critical model symbol exists in both outputs.
	if !strings.Contains(string(inPlaceData), "type User struct") {
		t.Fatalf("in-place output missing User struct")
	}
	if !strings.Contains(string(releaseData), "type User struct") {
		t.Fatalf("release output missing User struct")
	}
}
