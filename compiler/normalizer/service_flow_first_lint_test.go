package normalizer

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractServices_FlowFirstLint_CrudImplForbidden(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
CreateTender: {
	service: "tender"
	input: {}
	impls: go: {
		code: """
return nil, nil
"""
	}
}
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile CUE: %v", err)
	}

	n := New()
	var warnings []Warning
	n.WarningSink = func(w Warning) {
		warnings = append(warnings, w)
	}

	_, err := n.ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}

	found := false
	for _, w := range warnings {
		if w.Code == "FLOW_FIRST_IMPL_REQUIRED" {
			found = true
			if strings.ToLower(w.Severity) != "error" {
				t.Fatalf("expected error severity, got %s", w.Severity)
			}
			if !strings.Contains(w.CUEPath, "impls.go.code") {
				t.Fatalf("expected impls.go.code cue path, got %q", w.CUEPath)
			}
		}
	}
	if !found {
		t.Fatalf("expected FLOW_FIRST_IMPL_REQUIRED warning, got %+v", warnings)
	}
}

func TestExtractServices_FlowFirstLint_BypassAllowed(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
GetTender: {
	service: "tender"
	input: {}
	impls: go: {
		flowFirstBypass: true
		flowFirstBypassReason: "complex external SDK orchestration"
		code: """
return nil, nil
"""
	}
}
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile CUE: %v", err)
	}

	n := New()
	var warnings []Warning
	n.WarningSink = func(w Warning) {
		warnings = append(warnings, w)
	}

	_, err := n.ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}

	for _, w := range warnings {
		if w.Code == "FLOW_FIRST_IMPL_REQUIRED" || w.Code == "FLOW_FIRST_BYPASS_REASON_REQUIRED" {
			t.Fatalf("unexpected flow-first warning with bypass reason: %+v", w)
		}
	}
}

func TestExtractServices_FlowFirstLint_BypassRequiresReason(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
GetTender: {
	service: "tender"
	input: {}
	impls: go: {
		flowFirstBypass: true
		code: """
return nil, nil
"""
	}
}
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile CUE: %v", err)
	}

	n := New()
	var warnings []Warning
	n.WarningSink = func(w Warning) {
		warnings = append(warnings, w)
	}

	_, err := n.ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}

	found := false
	for _, w := range warnings {
		if w.Code == "FLOW_FIRST_BYPASS_REASON_REQUIRED" {
			found = true
			if strings.ToLower(w.Severity) != "error" {
				t.Fatalf("expected error severity, got %s", w.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected FLOW_FIRST_BYPASS_REASON_REQUIRED warning, got %+v", warnings)
	}
}

func TestExtractServices_FlowFirstLint_NonCrudImplAllowed(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
RecomputeWeightedScore: {
	service: "rating"
	input: {}
	impls: go: {
		code: """
return nil, nil
"""
	}
}
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile CUE: %v", err)
	}

	n := New()
	var warnings []Warning
	n.WarningSink = func(w Warning) {
		warnings = append(warnings, w)
	}

	_, err := n.ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}

	for _, w := range warnings {
		if w.Code == "FLOW_FIRST_IMPL_REQUIRED" {
			t.Fatalf("unexpected FLOW_FIRST_IMPL_REQUIRED warning: %+v", w)
		}
	}
}
