package normalizer

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractServices_ImplNamedReturnLint(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
Login: {
	service: "auth"
	output: {
		ok: bool
	}
	impls: go: {
		code: """
var resp port.LoginResponse
var err error
resp := port.LoginResponse{}
err := fmt.Errorf("boom")
return resp, err
"""
		imports: ["fmt"]
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

	requireCode := func(code string) {
		t.Helper()
		for _, w := range warnings {
			if w.Code == code {
				if w.Severity != "error" {
					t.Fatalf("expected severity error for %s, got %s", code, w.Severity)
				}
				if !strings.Contains(w.CUEPath, "impls.go.code") {
					t.Fatalf("expected CUEPath to include impls.go.code, got %q", w.CUEPath)
				}
				if strings.TrimSpace(w.Message) == "" {
					t.Fatalf("expected non-empty message for %s", code)
				}
				return
			}
		}
		t.Fatalf("expected warning code %s, got %+v", code, warnings)
	}

	requireCode("IMPL_NAMED_RETURN_RESP_VAR")
	requireCode("IMPL_NAMED_RETURN_ERR_VAR")
	requireCode("IMPL_NAMED_RETURN_RESP_SHORT_DECL")
	requireCode("IMPL_NAMED_RETURN_ERR_SHORT_DECL")
}

func TestExtractServices_ImplNamedReturnLint_AllowsAssignments(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
Login: {
	service: "auth"
	output: {
		ok: bool
	}
	impls: go: {
		code: """
resp = port.LoginResponse{Ok: true}
err = nil
return resp, err
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
		if strings.HasPrefix(w.Code, "IMPL_NAMED_RETURN_") {
			t.Fatalf("unexpected named-return lint warning: %+v", w)
		}
	}
}

func TestExtractServices_ImplNamedReturnLint_NotBypassedByFlowFirstBypass(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
GetTender: {
	service: "tender"
	output: {
		ok: bool
	}
	impls: go: {
		flowFirstBypass: true
		flowFirstBypassReason: "complex orchestration"
		code: """
var err error
err := fmt.Errorf("boom")
return resp, err
"""
		imports: ["fmt"]
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

	foundVar := false
	foundShort := false
	for _, w := range warnings {
		if w.Code == "IMPL_NAMED_RETURN_ERR_VAR" {
			foundVar = true
		}
		if w.Code == "IMPL_NAMED_RETURN_ERR_SHORT_DECL" {
			foundShort = true
		}
	}
	if !foundVar || !foundShort {
		t.Fatalf("expected named-return lint even with flowFirstBypass, got %+v", warnings)
	}
}
