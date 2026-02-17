package plan

import (
	"path/filepath"
	"testing"
)

func TestPlanRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.json")
	in := &BuildPlan{
		SchemaVersion:  "1",
		PlanVersion:    "0.1.0",
		GeneratedAtUTC: "2026-01-01T00:00:00Z",
		WorkspaceRoot:  dir,
		Status:         StatusOK,
	}
	if err := WritePlan(path, in); err != nil {
		t.Fatalf("WritePlan failed: %v", err)
	}
	out, err := ReadPlan(path)
	if err != nil {
		t.Fatalf("ReadPlan failed: %v", err)
	}
	if out.SchemaVersion != in.SchemaVersion {
		t.Fatalf("schema mismatch: got %q want %q", out.SchemaVersion, in.SchemaVersion)
	}
	if out.Status != in.Status {
		t.Fatalf("status mismatch: got %q want %q", out.Status, in.Status)
	}
}
