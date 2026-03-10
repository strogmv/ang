package flowsem

import "testing"

func TestPlanBuildAutomataSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{
		{Action: "mapping.Assign", Args: map[string]any{"to": "usecasesDoc", "declare": true, "value": "req.Usecases"}},
		{
			Action: "plan.BuildAutomata",
			Args: map[string]any{
				"input":  "usecasesDoc",
				"output": "automataDoc",
			},
		},
	})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestPlanBuildMicroPlanSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{
		{Action: "mapping.Assign", Args: map[string]any{"to": "usecasesDoc", "declare": true, "value": "req.Usecases"}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "automataDoc", "declare": true, "value": "req.Automata"}},
		{
			Action: "plan.BuildMicroPlan",
			Args: map[string]any{
				"usecases": "usecasesDoc",
				"automata": "automataDoc",
				"output":   "microPlanDoc",
			},
		},
	})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestCueEmitProjectSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{
		{Action: "mapping.Assign", Args: map[string]any{"to": "usecasesDoc", "declare": true, "value": "req.Usecases"}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "microPlanDoc", "declare": true, "value": "req.MicroPlan"}},
		{
			Action: "cue.EmitProject",
			Args: map[string]any{
				"usecases":   "usecasesDoc",
				"micro_plan": "microPlanDoc",
				"layout":     "single_file",
				"output":     "projectFiles",
			},
		},
	})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestStreamEmitSpec(t *testing.T) {
	t.Parallel()

	issues := ValidateWithOptions([]Step{
		{
			Action: "stream.Emit",
			Args: map[string]any{
				"data": `"{}"`,
			},
		},
	}, ValidateOptions{InStreamingMethod: true})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestCueValidateProjectSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{
		{Action: "mapping.Assign", Args: map[string]any{"to": "projectFiles", "declare": true, "value": "req.Files"}},
		{
			Action: "cue.ValidateProject",
			Args: map[string]any{
				"files":  "projectFiles",
				"output": "validation",
			},
		},
	})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestCueWriteProjectFilesSpec(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{
		{Action: "mapping.Assign", Args: map[string]any{"to": "projectFiles", "declare": true, "value": "req.Files"}},
		{
			Action: "cue.WriteProjectFiles",
			Args: map[string]any{
				"root":   "\"/tmp/project\"",
				"files":  "projectFiles",
				"output": "writeResult",
			},
		},
	})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}
