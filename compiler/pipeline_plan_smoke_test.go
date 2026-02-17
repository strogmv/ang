package compiler

import (
	"path/filepath"
	"testing"
)

func TestRunWithOptionsPlanSmoke(t *testing.T) {
	base := t.TempDir()
	outPlan := filepath.Join(base, ".ang", "plan.json")

	p, err := RunWithOptions(base, RunOptions{
		Phase:   PhasePlan,
		OutPlan: outPlan,
	})
	if err != nil {
		t.Fatalf("RunWithOptions(plan) failed: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil plan")
	}
	if p.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion mismatch: got %q want %q", p.SchemaVersion, SchemaVersion)
	}
	if p.PlanVersion != Version {
		t.Fatalf("planVersion mismatch: got %q want %q", p.PlanVersion, Version)
	}
}
