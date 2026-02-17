package plan

import "testing"

func TestValidatePlanRejectsUnsupportedSchema(t *testing.T) {
	p := &BuildPlan{
		SchemaVersion: "999",
		PlanVersion:   "0.1.0",
		WorkspaceRoot: ".",
		Status:        StatusOK,
	}
	if err := ValidatePlan(p); err == nil {
		t.Fatal("expected validation error for unsupported schema")
	}
}

func TestValidatePlanOK(t *testing.T) {
	p := &BuildPlan{
		SchemaVersion: "1",
		PlanVersion:   "0.1.0",
		WorkspaceRoot: ".",
		Status:        StatusWarn,
	}
	if err := ValidatePlan(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
