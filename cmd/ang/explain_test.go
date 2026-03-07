package main

import "testing"

func TestExplainAnyInput_Code(t *testing.T) {
	t.Parallel()
	items, err := explainAnyInput("MISSING_OUTPUT")
	if err != nil {
		t.Fatalf("explainAnyInput(code): %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Code != "MISSING_OUTPUT" {
		t.Fatalf("unexpected code: %s", items[0].Code)
	}
	if items[0].Title == "" || items[0].Description == "" {
		t.Fatalf("expected non-empty title/description")
	}
}

func TestDecodeDiagnostics_LintReportWarnings(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
  "ok": false,
  "warnings": [
    {
      "code": "MISSING_OUTPUT",
      "message": "repo.List missing output",
      "action": "repo.List",
      "step": 3,
      "cue_path": "services.Project.List.flow[2]"
    }
  ],
  "errors": []
}`)
	diags, err := decodeDiagnostics(raw)
	if err != nil {
		t.Fatalf("decodeDiagnostics: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diag, got %d", len(diags))
	}
	if diags[0].Code != "MISSING_OUTPUT" {
		t.Fatalf("unexpected code: %s", diags[0].Code)
	}
	if diags[0].Path != "services.Project.List.flow[2]" {
		t.Fatalf("unexpected path: %s", diags[0].Path)
	}
}

func TestDecodeDiagnostics_ContractErrorMessage(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
  "ok": false,
  "warnings": [],
  "errors": [
    {
      "message": "Validation FAILED: [CUE:CUE_PIPELINE_ERROR] run pipeline: boom"
    }
  ]
}`)
	diags, err := decodeDiagnostics(raw)
	if err != nil {
		t.Fatalf("decodeDiagnostics: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diag, got %d", len(diags))
	}
	if diags[0].Code != "CUE_PIPELINE_ERROR" {
		t.Fatalf("unexpected code: %s", diags[0].Code)
	}
	if diags[0].Stage != "CUE" {
		t.Fatalf("unexpected stage: %s", diags[0].Stage)
	}
}

func TestExplainInferredFlowMissing(t *testing.T) {
	t.Parallel()
	item := explainFromInput(explainInput{
		Code:    "E_FLOW_MISSING_OUTPUT",
		Message: "output is required",
		Action:  "flow.History.Get",
	})
	if len(item.Expected) == 0 {
		t.Fatalf("expected inferred expected field")
	}
	if item.DocAnchor == "" {
		t.Fatalf("expected doc anchor")
	}
}

func TestExplain_CUEFieldNotAllowedPattern(t *testing.T) {
	t.Parallel()
	item := explainFromInput(explainInput{
		Code:    "UNKNOWN_ERROR",
		Message: `field "soruce" not allowed`,
	})
	if item.Title != "Typo in flow step field name" {
		t.Fatalf("unexpected title: %s", item.Title)
	}
	if item.Fix == "" || item.DocAnchor == "" {
		t.Fatalf("expected actionable fix and doc anchor")
	}
}

func TestExplain_CUECannotUnifyRepoGetPattern(t *testing.T) {
	t.Parallel()
	item := explainFromInput(explainInput{
		Code:    "UNKNOWN_ERROR",
		Message: "cannot unify struct with #RepoGetStep",
	})
	if item.Title != "Wrong repo step structure" {
		t.Fatalf("unexpected title: %s", item.Title)
	}
	if item.ActionRef != "repo.Get" {
		t.Fatalf("unexpected action ref: %s", item.ActionRef)
	}
}

func TestExplain_CUEConflictingValuesStringPattern(t *testing.T) {
	t.Parallel()
	item := explainFromInput(explainInput{
		Code:    "UNKNOWN_ERROR",
		Message: "conflicting values int and string",
	})
	if item.Title != "Wrong value type in flow step" {
		t.Fatalf("unexpected title: %s", item.Title)
	}
	if item.Fix == "" {
		t.Fatalf("expected fix")
	}
}

func TestExplain_CUEDoesNotExistPattern(t *testing.T) {
	t.Parallel()
	item := explainFromInput(explainInput{
		Code:    "UNKNOWN_ERROR",
		Message: "reference req.CompanyID does not exist",
	})
	if item.Title != "Reference to undefined variable" {
		t.Fatalf("unexpected title: %s", item.Title)
	}
	if item.Fix == "" {
		t.Fatalf("expected fix")
	}
}

func TestExplain_CUEInvalidInterpolationPattern(t *testing.T) {
	t.Parallel()
	item := explainFromInput(explainInput{
		Code:    "UNKNOWN_ERROR",
		Message: "invalid interpolation: reference _field not found",
	})
	if item.Title != "Invalid CUE interpolation" {
		t.Fatalf("unexpected title: %s", item.Title)
	}
	if item.Fix == "" || item.DocAnchor == "" {
		t.Fatalf("expected actionable fix and doc anchor")
	}
}

func TestExplain_CUEIncompleteValuePattern(t *testing.T) {
	t.Parallel()
	item := explainFromInput(explainInput{
		Code:    "UNKNOWN_ERROR",
		Message: "incomplete value string:",
	})
	if item.Title != "Incomplete CUE value" {
		t.Fatalf("unexpected title: %s", item.Title)
	}
	if item.Fix == "" {
		t.Fatalf("expected fix")
	}
}

func TestExplain_CUEListLengthMismatchPattern(t *testing.T) {
	t.Parallel()
	item := explainFromInput(explainInput{
		Code:    "UNKNOWN_ERROR",
		Message: "incompatible list lengths: 2 and 3",
	})
	if item.Title != "List length mismatch" {
		t.Fatalf("unexpected title: %s", item.Title)
	}
	if item.Fix == "" {
		t.Fatalf("expected fix")
	}
}
