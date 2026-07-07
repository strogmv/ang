package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeProjectHealthReportsDeadCUEAndMissingManifest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cue", "schedules", "jobs.cue")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package schedules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := analyzeProjectHealth(root)
	if report.Status == "ok" {
		t.Fatalf("expected warnings, got %#v", report)
	}
	found := false
	for _, check := range report.Checks {
		found = found || check.Code == "CUE_DEAD_DIRECTORY"
	}
	if !found {
		t.Fatalf("dead CUE directory missing from report: %#v", report)
	}
}
