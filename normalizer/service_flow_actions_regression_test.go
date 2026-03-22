package normalizer

import "testing"

func TestValidateFlowSteps_ListFindReportsMissingCondition(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{Action: "list.Find", Args: map[string]any{"from": "items", "output": "match"}, File: "cue/api/test.cue", Line: 10, Column: 1},
	}

	got := validateFlowSteps("TestOp", "test", steps, nil, nil, nil, "", nil)
	if !hasFlowWarningCode(got, "MISSING_CONDITION") {
		t.Fatalf("expected MISSING_CONDITION warning, got %#v", got)
	}
}

func TestValidateFlowSteps_ValueCoalesceReportsMissingValues(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{Action: "value.Coalesce", Args: map[string]any{"output": "displayName"}, File: "cue/api/test.cue", Line: 10, Column: 1},
	}

	got := validateFlowSteps("TestOp", "test", steps, nil, nil, nil, "", nil)
	if !hasFlowWarningCode(got, "MISSING_VALUES") {
		t.Fatalf("expected MISSING_VALUES warning, got %#v", got)
	}
}

func TestValidateFlowSteps_ListFindScopeCatchesUndeclaredVar(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{Action: "list.New", Args: map[string]any{"output": "items", "type": "[]string"}, File: "cue/api/test.cue", Line: 5, Column: 1},
		{Action: "list.Find", Args: map[string]any{"from": "items", "as": "item", "condition": "missing.ID == item.ID", "output": "match"}, File: "cue/api/test.cue", Line: 10, Column: 1},
	}

	got := validateFlowSteps("TestOp", "test", steps, nil, nil, nil, "", nil)
	if !hasFlowWarningCode(got, "UNDECLARED_FLOW_VAR") {
		t.Fatalf("expected UNDECLARED_FLOW_VAR warning, got %#v", got)
	}
}

func hasFlowWarningCode(got []FlowWarning, code string) bool {
	for _, w := range got {
		if w.Code == code {
			return true
		}
	}
	return false
}
