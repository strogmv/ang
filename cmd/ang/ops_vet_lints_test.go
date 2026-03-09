package main

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

// ── lintFlowTooLarge ─────────────────────────────────────────────────────────

func TestLintFlowTooLarge_OK(t *testing.T) {
	flow := make([]normalizer.FlowStep, flowTooLargeThreshold)
	got := lintFlowTooLarge("users.CreateUser", flow)
	if len(got) != 0 {
		t.Errorf("expected no warnings for flow=%d, got %d", len(flow), len(got))
	}
}

func TestLintFlowTooLarge_Triggers(t *testing.T) {
	flow := make([]normalizer.FlowStep, flowTooLargeThreshold+1)
	got := lintFlowTooLarge("users.CreateUser", flow)
	if len(got) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(got))
	}
	w := got[0]
	if w.Code != codeFlowTooLarge {
		t.Errorf("code = %q, want %q", w.Code, codeFlowTooLarge)
	}
	if w.Severity != "error" {
		t.Errorf("severity = %q, want error", w.Severity)
	}
	if w.Hint == "" {
		t.Error("hint must be non-empty")
	}
}

// ── lintHTTPNoTimeout ────────────────────────────────────────────────────────

func TestLintHTTPNoTimeout_NoHTTPSteps(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "repo.Save", Args: map[string]any{"source": "User"}},
	}
	got := lintHTTPNoTimeout("users.CreateUser", flow, lintProfileMini)
	if len(got) != 0 {
		t.Errorf("expected no warnings, got %d", len(got))
	}
}

func TestLintHTTPNoTimeout_WithTimeout(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "http.Request", Args: map[string]any{"url": "https://api.example.com", "timeout": "5s"}},
	}
	got := lintHTTPNoTimeout("users.CreateUser", flow, lintProfileMini)
	if len(got) != 0 {
		t.Errorf("expected no warnings (timeout present), got %d", len(got))
	}
}

func TestLintHTTPNoTimeout_Missing(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "http.Request", File: "cue/api/users.cue", Line: 12, Args: map[string]any{"url": "https://api.example.com"}},
	}
	got := lintHTTPNoTimeout("users.CreateUser", flow, lintProfileMini)
	if len(got) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(got))
	}
	w := got[0]
	if w.Code != codeHTTPNoTimeout {
		t.Errorf("code = %q, want %q", w.Code, codeHTTPNoTimeout)
	}
	if w.Severity != "warn" {
		t.Errorf("severity = %q, want warn", w.Severity)
	}
	if w.Step != 1 {
		t.Errorf("step = %d, want 1", w.Step)
	}
	if !w.CanAutoApply {
		t.Errorf("expected CanAutoApply=true")
	}
	if len(w.SuggestedFix) == 0 {
		t.Fatalf("expected suggested_fix")
	}
	if w.SuggestedFix[0].Op != "merge" {
		t.Errorf("suggested_fix[0].op=%q, want merge", w.SuggestedFix[0].Op)
	}
}

func TestLintHTTPNoTimeout_MultipleSteps(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "repo.Find", Args: map[string]any{}},
		{Action: "http.Call", Args: map[string]any{"url": "https://x"}},
		{Action: "http.Request", Args: map[string]any{"url": "https://y", "timeout": "3s"}},
	}
	got := lintHTTPNoTimeout("users.CreateUser", flow, lintProfileMini)
	if len(got) != 1 {
		t.Fatalf("expected 1 warning (only http.Call missing timeout), got %d", len(got))
	}
	if got[0].Action != "http.Call" {
		t.Errorf("action = %q, want http.Call", got[0].Action)
	}
}

func TestLintHTTPNoTimeout_InsideFlowTimeout_NoWarning(t *testing.T) {
	flow := []normalizer.FlowStep{
		{
			Action: "flow.Timeout",
			Args: map[string]any{
				"duration": "5s",
				"_do": []normalizer.FlowStep{
					{Action: "http.Request", Args: map[string]any{"method": "GET", "url": "https://api.example.com"}},
				},
			},
		},
	}
	got := lintHTTPNoTimeout("users.CreateUser", flow, lintProfileMini)
	if len(got) != 0 {
		t.Fatalf("expected no warnings for http call wrapped in flow.Timeout, got %d", len(got))
	}
}

// ── lintOutboxPreferred ──────────────────────────────────────────────────────

func TestLintOutboxPreferred_NoPublish(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "repo.Save"},
	}
	got := lintOutboxPreferred("users.CreateUser", flow, lintProfileMini)
	if len(got) != 0 {
		t.Errorf("expected no warnings (no publish), got %d", len(got))
	}
}

func TestLintOutboxPreferred_WithOutbox(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "repo.Save"},
		{Action: "event.Outbox"},
	}
	got := lintOutboxPreferred("users.CreateUser", flow, lintProfileMini)
	if len(got) != 0 {
		t.Errorf("expected no warnings (outbox present), got %d", len(got))
	}
}

func TestLintOutboxPreferred_Triggers(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "repo.Save"},
		{Action: "event.Publish", File: "cue/api/users.cue", Line: 14},
	}
	got := lintOutboxPreferred("users.CreateUser", flow, lintProfileMini)
	if len(got) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(got))
	}
	w := got[0]
	if w.Code != codeOutboxPreferred {
		t.Errorf("code = %q, want %q", w.Code, codeOutboxPreferred)
	}
	if w.Severity != "warn" {
		t.Errorf("severity = %q, want warn", w.Severity)
	}
	if w.CanAutoApply {
		t.Errorf("expected outbox lint to stay manual")
	}
	if len(w.SuggestedFix) == 0 {
		t.Fatalf("expected suggested_fix for manual guidance")
	}
}

// ── new layer-2 lints ───────────────────────────────────────────────────────

func TestLintInvalidNestingKeys_Triggers(t *testing.T) {
	flow := []normalizer.FlowStep{
		{
			Action: "mapping.Assign",
			Args: map[string]any{
				"to":    "resp.ID",
				"value": "obj.ID",
				"_then": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "x"}}},
			},
		},
	}
	got := lintInvalidNestingKeys("users.CreateUser", flow)
	if len(got) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(got))
	}
	if got[0].Code != codeInvalidNestingKeys {
		t.Fatalf("code = %s, want %s", got[0].Code, codeInvalidNestingKeys)
	}
}

func TestLintDBWriteWithoutTxWhenRequired_Triggers(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "repo.Save", Args: map[string]any{"source": "User", "input": "user"}},
		{Action: "repo.Update", Args: map[string]any{"source": "User", "input": "user"}},
	}
	got := lintDBWriteWithoutTxWhenRequired("users.CreateUser", flow)
	if len(got) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(got))
	}
	if got[0].Code != codeDBWriteWithoutTxWhenRequired {
		t.Fatalf("code = %s, want %s", got[0].Code, codeDBWriteWithoutTxWhenRequired)
	}
}

func TestLintDBWriteWithoutTxWhenRequired_InsideTx_NoWarning(t *testing.T) {
	flow := []normalizer.FlowStep{
		{
			Action: "tx.Block",
			Args: map[string]any{
				"_do": []normalizer.FlowStep{
					{Action: "repo.Save", Args: map[string]any{"source": "User", "input": "user"}},
					{Action: "repo.Update", Args: map[string]any{"source": "User", "input": "user"}},
				},
			},
		},
	}
	got := lintDBWriteWithoutTxWhenRequired("users.CreateUser", flow)
	if len(got) != 0 {
		t.Fatalf("expected no warning, got %d", len(got))
	}
}

func TestLintMutatingEndpointWithoutIdempotency_Prod_Error(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "repo.Save", Args: map[string]any{"source": "Order", "input": "order"}},
	}
	endpoint := normalizer.Endpoint{
		Method:      "POST",
		Path:        "/orders",
		ServiceName: "Orders",
		RPC:         "CreateOrder",
	}
	got := lintMutatingEndpointWithoutIdempotency("Orders.CreateOrder", endpoint, flow, lintProfileProd)
	if len(got) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(got))
	}
	if got[0].Severity != "error" {
		t.Fatalf("severity = %s, want error", got[0].Severity)
	}
}

func TestLintProtectedOperationWithoutAuthz_Triggers(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "repo.Save", Args: map[string]any{"source": "Order", "input": "order"}},
	}
	endpoint := normalizer.Endpoint{
		Method:      "POST",
		Path:        "/orders",
		ServiceName: "Orders",
		RPC:         "CreateOrder",
	}
	got := lintProtectedOperationWithoutAuthz("Orders.CreateOrder", endpoint, flow, lintProfileProd)
	if len(got) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(got))
	}
	if got[0].Code != codeProtectedOperationWithoutAuth {
		t.Fatalf("code = %s, want %s", got[0].Code, codeProtectedOperationWithoutAuth)
	}
}

func TestLintProtectedOperationWithoutAuthz_WithEndpointAuth_NoWarning(t *testing.T) {
	flow := []normalizer.FlowStep{
		{Action: "repo.Save", Args: map[string]any{"source": "Order", "input": "order"}},
	}
	endpoint := normalizer.Endpoint{
		Method:      "POST",
		Path:        "/orders",
		ServiceName: "Orders",
		RPC:         "CreateOrder",
		AuthType:    "jwt",
	}
	got := lintProtectedOperationWithoutAuthz("Orders.CreateOrder", endpoint, flow, lintProfileProd)
	if len(got) != 0 {
		t.Fatalf("expected no warnings, got %d", len(got))
	}
}

// ── findSimilarActions ───────────────────────────────────────────────────────

func TestFindSimilarActions_SameNamespace(t *testing.T) {
	catalog := []string{
		"http.Request", "http.Call", "http.Get", "http.Post",
		"repo.Save", "repo.Find",
	}
	got := findSimilarActions("http.Reqeust", catalog)
	if len(got) == 0 {
		t.Fatal("expected suggestions, got none")
	}
	// http.Request should be top suggestion (closest to http.Reqeust)
	if got[0] != "http.Request" {
		t.Errorf("top suggestion = %q, want http.Request", got[0])
	}
	// All suggestions should be http.* (same namespace)
	for _, s := range got {
		if !isHTTPNamespace(s) {
			t.Errorf("suggestion %q is not in http namespace", s)
		}
	}
}

func TestFindSimilarActions_NoNamespace(t *testing.T) {
	catalog := []string{"repo.Save", "repo.Find", "http.Request"}
	got := findSimilarActions("repoSave", catalog)
	// should return something without panicking
	_ = got
}

func TestFindSimilarActions_EmptyInput(t *testing.T) {
	catalog := []string{"repo.Save"}
	got := findSimilarActions("", catalog)
	_ = got // should not panic
}

// ── editDistance ─────────────────────────────────────────────────────────────

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"kitten", "sitting", 3},
		{"http.Request", "http.Request", 0},
		{"http.Reqeust", "http.Request", 2},
		{"repo.Sav", "repo.Save", 1},
	}
	for _, c := range cases {
		got := editDistance(c.a, c.b)
		if got != c.want {
			t.Errorf("editDistance(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// ── computeVetPath ───────────────────────────────────────────────────────────

func TestComputeVetPath(t *testing.T) {
	cases := []struct {
		w    normalizer.Warning
		want string
	}{
		{
			normalizer.Warning{File: "cue/api/users.cue", CUEPath: "api.CreateUser.flow[3]"},
			"cue/api/users.cue:CreateUser.flow[3]",
		},
		{
			normalizer.Warning{File: "cue/api/users.cue", Step: 4},
			"cue/api/users.cue:flow[3]",
		},
		{
			normalizer.Warning{CUEPath: "api.CreateUser.flow[3]"},
			"CreateUser.flow[3]",
		},
		{
			normalizer.Warning{File: "cue/api/users.cue"},
			"cue/api/users.cue",
		},
	}
	for _, c := range cases {
		got := computeVetPath(c.w)
		if got != c.want {
			t.Errorf("computeVetPath(%+v) = %q, want %q", c.w, got, c.want)
		}
	}
}

// ── enrichUnknownActionDiags ─────────────────────────────────────────────────

func TestEnrichUnknownActionDiags_AddsHint(t *testing.T) {
	diags := []normalizer.Warning{
		{
			Code:    "E_FLOW_UNKNOWN_ACTION",
			Message: "Unknown action 'http.Reqeust'",
			Action:  "http.Reqeust",
			File:    "cue/api/users.cue",
			Line:    10,
			CUEPath: "api.CreateUser.flow[0]",
		},
	}
	enriched := enrichUnknownActionDiags(diags)
	if len(enriched[0].Allowed) == 0 {
		t.Error("expected Allowed to be populated")
	}
	if enriched[0].Hint == "" {
		t.Error("expected Hint to be populated")
	}
	// hint should contain "Did you mean"
	if !containsCI(enriched[0].Hint, "did you mean") {
		t.Errorf("hint %q should contain 'Did you mean'", enriched[0].Hint)
	}
	if !enriched[0].CanAutoApply {
		t.Errorf("expected CanAutoApply=true")
	}
	if len(enriched[0].SuggestedFix) == 0 {
		t.Fatalf("expected suggested_fix")
	}
	if enriched[0].SuggestedFix[0].Op != "merge" {
		t.Errorf("suggested_fix[0].op=%q, want merge", enriched[0].SuggestedFix[0].Op)
	}
}

func TestEnrichUnknownActionDiags_NonUnknownUnchanged(t *testing.T) {
	diags := []normalizer.Warning{
		{Code: "E_FLOW_MISSING_OUTPUT", Message: "missing output", Action: "repo.Find"},
	}
	enriched := enrichUnknownActionDiags(diags)
	if len(enriched[0].Allowed) != 0 {
		t.Error("non-UNKNOWN_ACTION warning should not be enriched")
	}
}

// ── runSemanticLints integration ─────────────────────────────────────────────

func TestRunSemanticLints_Empty(t *testing.T) {
	got := runSemanticLints(nil, nil, lintProfileMini)
	if len(got) != 0 {
		t.Errorf("expected no warnings for empty services, got %d", len(got))
	}
}

func TestRunSemanticLints_CombinedRules(t *testing.T) {
	// Service with two violations: large flow AND http without timeout
	bigFlow := make([]normalizer.FlowStep, flowTooLargeThreshold+1)
	bigFlow[0] = normalizer.FlowStep{Action: "http.Request", Args: map[string]any{"url": "x"}}

	services := []normalizer.Service{{
		Name: "users",
		Methods: []normalizer.Method{{
			Name: "HeavyOp",
			Flow: bigFlow,
		}},
	}}
	got := runSemanticLints(services, nil, lintProfileMini)
	codes := make(map[string]bool)
	for _, w := range got {
		codes[w.Code] = true
	}
	if !codes[codeFlowTooLarge] {
		t.Error("missing E_FLOW_TOO_LARGE")
	}
	if !codes[codeHTTPNoTimeout] {
		t.Error("missing W_FLOW_HTTP_NO_TIMEOUT")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func isHTTPNamespace(s string) bool {
	return len(s) > 5 && s[:5] == "http."
}

func containsCI(s, sub string) bool {
	ls := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		ls[i] = c
	}
	lsub := make([]byte, len(sub))
	for i := range sub {
		c := sub[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		lsub[i] = c
	}
	return len(ls) >= len(lsub) && func() bool {
		for i := 0; i <= len(ls)-len(lsub); i++ {
			match := true
			for j := range lsub {
				if ls[i+j] != lsub[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
		return false
	}()
}
