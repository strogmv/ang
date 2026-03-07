package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpsSchemaJSON(t *testing.T) {
	t.Parallel()
	schema := buildOpsSchema()
	if schema.Schema != "ang/ops-schema/v2" {
		t.Fatalf("unexpected schema: %s", schema.Schema)
	}
	if schema.Version == "" {
		t.Fatal("expected non-empty version")
	}
	if len(schema.OpFileGlobs) == 0 {
		t.Fatal("expected non-empty op_file_globs")
	}
	if schema.OperationType == "" {
		t.Fatal("expected non-empty operation_type")
	}
	if len(schema.Modes) == 0 {
		t.Fatal("expected non-empty modes")
	}
	if len(schema.Operation.RequiredFields) == 0 {
		t.Fatal("expected non-empty required_fields")
	}
	// service, input, output must be required
	names := make(map[string]bool)
	for _, f := range schema.Operation.RequiredFields {
		names[f.Name] = true
	}
	for _, req := range []string{"service", "input", "output"} {
		if !names[req] {
			t.Errorf("missing required field: %s", req)
		}
	}
	if len(schema.Operation.OptionalFields) == 0 {
		t.Fatal("expected non-empty optional_fields")
	}
	if len(schema.Rules) == 0 {
		t.Fatal("expected non-empty rules")
	}
	// R_OP_MODE_XOR must be present
	hasXOR := false
	for _, r := range schema.Rules {
		if r.Code == "R_OP_MODE_XOR" {
			hasXOR = true
		}
		if r.Code == "" {
			t.Errorf("rule has empty code: %+v", r)
		}
		if r.Desc == "" {
			t.Errorf("rule %s has empty desc", r.Code)
		}
	}
	if !hasXOR {
		t.Error("missing rule R_OP_MODE_XOR")
	}
	if schema.Examples.Minimal == "" {
		t.Fatal("expected non-empty examples.minimal")
	}
	if schema.Examples.Full == "" {
		t.Fatal("expected non-empty examples.full")
	}
	if len(schema.Examples.Invalid) == 0 {
		t.Fatal("expected non-empty examples.invalid")
	}
	for _, inv := range schema.Examples.Invalid {
		if inv.Violation == "" {
			t.Errorf("invalid example %q has empty violation", inv.Label)
		}
		if inv.CUE == "" {
			t.Errorf("invalid example %q has empty cue", inv.Label)
		}
	}
	if schema.ActionsRef == "" {
		t.Fatal("expected non-empty actions_ref")
	}
}

func TestOpsSchemaJSONMarshal(t *testing.T) {
	t.Parallel()
	schema := buildOpsSchema()
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var roundtrip opsSchemaEnvelope
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if roundtrip.Schema != schema.Schema {
		t.Fatalf("schema mismatch: got %s", roundtrip.Schema)
	}
}

func TestOpsSchemaCUE(t *testing.T) {
	t.Parallel()
	schema := buildOpsSchema()
	cue := renderOpsSchemaCUE(schema)
	if !strings.Contains(cue, "#Operation") {
		t.Error("CUE output missing #Operation")
	}
	if !strings.Contains(cue, "service:") {
		t.Error("CUE output missing service field")
	}
	if !strings.Contains(cue, "flow?:") {
		t.Error("CUE output missing optional flow field")
	}
}

func TestDecodeDiagnostics_DiagnosticsFormat(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
  "schema": "ang/diags/v1",
  "valid": false,
  "diagnostics": [
    {
      "code": "MISSING_OUTPUT",
      "severity": "error",
      "message": "repo.Find missing output",
      "cue_path": "api.CreatePost.flow[1]",
      "action": "repo.Find",
      "file": "cue/api/posts.cue",
      "line": 25,
      "hint": "Add output field"
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
	if diags[0].Code != "MISSING_OUTPUT" {
		t.Fatalf("unexpected code: %s", diags[0].Code)
	}
	if diags[0].Path != "api.CreatePost.flow[1]" {
		t.Fatalf("unexpected path: %s", diags[0].Path)
	}
	if diags[0].Hint != "Add output field" {
		t.Fatalf("unexpected hint: %s", diags[0].Hint)
	}
}

func TestExplainNewCodes(t *testing.T) {
	t.Parallel()
	codes := []string{
		"MISSING_CONTENT",
		"MISSING_APPROVERS",
		"INVALID_APPROVERS_TYPE",
		"INVALID_POLICY",
		"MISSING_BLOCK",
		"MISSING_FIELD",
	}
	for _, code := range codes {
		code := code
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			item := explainFromInput(explainInput{Code: code})
			if item.Title == "Unknown diagnostic code" {
				t.Errorf("code %s has no entry in explainDB", code)
			}
			if item.Description == "" {
				t.Errorf("code %s has empty description", code)
			}
			if item.Hint == "" {
				t.Errorf("code %s has empty hint", code)
			}
		})
	}
}
