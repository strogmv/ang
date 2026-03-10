package flowsem

import "testing"

func TestPlanBuildAutomataSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{{
		Action: "plan.BuildAutomata",
		Args: map[string]any{
			"input":  "usecasesDoc",
			"output": "automataDoc",
		},
	}})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestPlanBuildMicroPlanSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{{
		Action: "plan.BuildMicroPlan",
		Args: map[string]any{
			"usecases": "usecasesDoc",
			"automata": "automataDoc",
			"output":   "microPlanDoc",
		},
	}})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestCueEmitProjectSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{{
		Action: "cue.EmitProject",
		Args: map[string]any{
			"usecases":   "usecasesDoc",
			"micro_plan": "microPlanDoc",
			"output":     "projectFiles",
		},
	}})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestCueValidateProjectSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{{
		Action: "cue.ValidateProject",
		Args: map[string]any{
			"files":  "projectFiles",
			"output": "validation",
		},
	}})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestCueWriteProjectFilesSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{{
		Action: "cue.WriteProjectFiles",
		Args: map[string]any{
			"root":   "\"/tmp/project\"",
			"files":  "projectFiles",
			"output": "writeResult",
		},
	}})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}
