package main

import (
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func makeStep(action string, args map[string]any) normalizer.FlowStep {
	if args == nil {
		args = map[string]any{}
	}
	return normalizer.FlowStep{Action: action, Args: args}
}

func emptyEndpoint() normalizer.Endpoint {
	return normalizer.Endpoint{Method: "POST"}
}

func authedEndpoint() normalizer.Endpoint {
	return normalizer.Endpoint{Method: "POST", AuthType: "jwt"}
}

// ─────────────────────────────────────────────────────────────────────────────
// logosFor
// ─────────────────────────────────────────────────────────────────────────────

func TestLogosFor_KnownActions(t *testing.T) {
	t.Parallel()
	cases := []string{
		"auth.Verify", "auth.RequireRole", "rbac.CheckPermission",
		"repo.Save", "repo.Get", "repo.Delete",
		"event.Outbox", "event.Publish",
		"http.Request", "http.Get",
		"tx.Block", "flow.Try", "flow.Saga",
		"logic.Validate", "mapping.Map",
	}
	for _, name := range cases {
		l := logosFor(name)
		if l == nil {
			t.Errorf("logosFor(%q) = nil, want logos", name)
		}
	}
}

func TestLogosFor_UnknownAction(t *testing.T) {
	t.Parallel()
	if l := logosFor("unknown.Action"); l != nil {
		t.Errorf("expected nil logos for unknown action, got %+v", l)
	}
}

func TestLogosFor_AuthProvidesCapability(t *testing.T) {
	t.Parallel()
	l := logosFor("auth.Verify")
	if l == nil {
		t.Fatal("expected logos for auth.Verify")
	}
	found := false
	for _, p := range l.Provides {
		if p == capAuthVerified {
			found = true
		}
	}
	if !found {
		t.Errorf("auth.Verify should provide %q, got %v", capAuthVerified, l.Provides)
	}
}

func TestLogosFor_RepoSaveEffect(t *testing.T) {
	t.Parallel()
	l := logosFor("repo.Save")
	if l == nil {
		t.Fatal("expected logos for repo.Save")
	}
	found := false
	for _, e := range l.Effects {
		if e == effectDBWrite {
			found = true
		}
	}
	if !found {
		t.Errorf("repo.Save should have %q effect, got %v", effectDBWrite, l.Effects)
	}
}

func TestLogosFor_HTTPRequestCompensatable(t *testing.T) {
	t.Parallel()
	l := logosFor("http.Request")
	if l == nil {
		t.Fatal("expected logos for http.Request")
	}
	if l.FailureMode != "compensatable" {
		t.Errorf("http.Request FailureMode = %q, want compensatable", l.FailureMode)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildFlowGraph / capabilitySetUpTo
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildFlowGraph_LinearFlow(t *testing.T) {
	t.Parallel()
	flow := []normalizer.FlowStep{
		makeStep("auth.Verify", nil),
		makeStep("repo.Get", nil),
		makeStep("repo.Save", nil),
	}
	g := buildFlowGraph(flow)
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("linear flow should have no edges, got %d", len(g.Edges))
	}
}

func TestBuildFlowGraph_NestedBlock(t *testing.T) {
	t.Parallel()
	inner := []normalizer.FlowStep{makeStep("repo.Save", nil)}
	flow := []normalizer.FlowStep{
		makeStep("auth.Verify", nil),
		makeStep("tx.Block", map[string]any{"_do": inner}),
	}
	g := buildFlowGraph(flow)
	// Nodes: auth.Verify, tx.Block, repo.Save = 3
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 nodes (auth+tx+save), got %d", len(g.Nodes))
	}
	// Should have 1 control edge: tx.Block → repo.Save
	controlEdges := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeControl {
			controlEdges++
		}
	}
	if controlEdges != 1 {
		t.Errorf("expected 1 control edge, got %d", controlEdges)
	}
}

func TestCapabilitySetUpTo_Auth(t *testing.T) {
	t.Parallel()
	flow := []normalizer.FlowStep{
		makeStep("auth.Verify", nil),
		makeStep("repo.Save", nil),
	}
	g := buildFlowGraph(flow)
	// At index 0 (auth.Verify) — no caps yet
	caps0 := capabilitySetUpTo(g.Nodes, 0)
	if caps0[capAuthVerified] {
		t.Error("cap should not be present before auth.Verify executes")
	}
	// At index 1 (repo.Save) — auth.Verify already ran
	caps1 := capabilitySetUpTo(g.Nodes, 1)
	if !caps1[capAuthVerified] {
		t.Error("cap should be present after auth.Verify")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// L3 lint: lintChainMissingAuthGate
// ─────────────────────────────────────────────────────────────────────────────

func TestLintChainMissingAuthGate_NoWarning_WhenEndpointAuthed(t *testing.T) {
	t.Parallel()
	flow := []normalizer.FlowStep{makeStep("repo.Save", nil)}
	g := buildFlowGraph(flow)
	warns := lintChainMissingAuthGate("Svc.Op", g, authedEndpoint(), lintProfileMini)
	if len(warns) != 0 {
		t.Errorf("expected no warnings when endpoint has auth, got %d", len(warns))
	}
}

func TestLintChainMissingAuthGate_NoWarning_WhenAuthInFlow(t *testing.T) {
	t.Parallel()
	flow := []normalizer.FlowStep{
		makeStep("auth.RequireRole", nil),
		makeStep("repo.Save", nil),
	}
	g := buildFlowGraph(flow)
	warns := lintChainMissingAuthGate("Svc.Op", g, emptyEndpoint(), lintProfileMini)
	if len(warns) != 0 {
		t.Errorf("expected no warnings when auth precedes write, got %d", len(warns))
	}
}

func TestLintChainMissingAuthGate_Warns_WhenNoAuth(t *testing.T) {
	t.Parallel()
	flow := []normalizer.FlowStep{
		makeStep("mapping.Map", nil),
		makeStep("repo.Save", nil),
	}
	g := buildFlowGraph(flow)
	warns := lintChainMissingAuthGate("Svc.Op", g, emptyEndpoint(), lintProfileMini)
	if len(warns) == 0 {
		t.Error("expected warning for write without auth gate")
	}
	if warns[0].Code != codeChainMissingAuthGate {
		t.Errorf("unexpected code: %s", warns[0].Code)
	}
}

func TestLintChainMissingAuthGate_SkipsGetEndpoint(t *testing.T) {
	t.Parallel()
	flow := []normalizer.FlowStep{makeStep("repo.Save", nil)}
	g := buildFlowGraph(flow)
	ep := normalizer.Endpoint{Method: "GET"}
	warns := lintChainMissingAuthGate("Svc.Op", g, ep, lintProfileMini)
	if len(warns) != 0 {
		t.Errorf("GET endpoint should not trigger auth-gate lint, got %d warns", len(warns))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// L3 lint: lintChainExternalWithoutCompensation
// ─────────────────────────────────────────────────────────────────────────────

func TestLintChainExternalWithoutCompensation_Warns(t *testing.T) {
	t.Parallel()
	flow := []normalizer.FlowStep{
		makeStep("http.Request", nil),
		makeStep("repo.Save", nil),
	}
	g := buildFlowGraph(flow)
	warns := lintChainExternalWithoutCompensation("Svc.Op", g)
	if len(warns) == 0 {
		t.Error("expected warning for http.Request without try/saga")
	}
	if warns[0].Code != codeChainExternalWithoutCompensation {
		t.Errorf("unexpected code: %s", warns[0].Code)
	}
}

func TestLintChainExternalWithoutCompensation_NoWarn_WithFlowTry(t *testing.T) {
	t.Parallel()
	inner := []normalizer.FlowStep{makeStep("http.Request", nil)}
	flow := []normalizer.FlowStep{
		makeStep("flow.Try", map[string]any{"_do": inner}),
	}
	g := buildFlowGraph(flow)
	warns := lintChainExternalWithoutCompensation("Svc.Op", g)
	if len(warns) != 0 {
		t.Errorf("expected no warning when http.Request is inside flow.Try, got %d", len(warns))
	}
}

func TestLintChainExternalWithoutCompensation_NoWarn_HTTPGet(t *testing.T) {
	t.Parallel()
	// http.Get is retryable, not compensatable
	flow := []normalizer.FlowStep{makeStep("http.Get", nil)}
	g := buildFlowGraph(flow)
	warns := lintChainExternalWithoutCompensation("Svc.Op", g)
	if len(warns) != 0 {
		t.Errorf("http.Get should not trigger compensation lint, got %d warns", len(warns))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// L3 lint: lintChainDBWriteBeforeValidation
// ─────────────────────────────────────────────────────────────────────────────

func TestLintChainDBWriteBeforeValidation_Warns(t *testing.T) {
	t.Parallel()
	flow := []normalizer.FlowStep{
		makeStep("repo.Save", nil),
		makeStep("logic.Validate", nil), // too late
	}
	g := buildFlowGraph(flow)
	warns := lintChainDBWriteBeforeValidation("Svc.Op", g)
	if len(warns) == 0 {
		t.Error("expected warning for DB write before validation")
	}
	if warns[0].Code != codeChainDBWriteBeforeValidation {
		t.Errorf("unexpected code: %s", warns[0].Code)
	}
}

func TestLintChainDBWriteBeforeValidation_NoWarn_ValidationFirst(t *testing.T) {
	t.Parallel()
	flow := []normalizer.FlowStep{
		makeStep("logic.Validate", nil),
		makeStep("repo.Save", nil),
	}
	g := buildFlowGraph(flow)
	warns := lintChainDBWriteBeforeValidation("Svc.Op", g)
	if len(warns) != 0 {
		t.Errorf("expected no warning when validation precedes write, got %d", len(warns))
	}
}

func TestLintChainDBWriteBeforeValidation_NoWarn_NoValidation(t *testing.T) {
	t.Parallel()
	// If there's no validation step at all, this lint is silent
	// (other lints handle missing validation separately)
	flow := []normalizer.FlowStep{makeStep("repo.Save", nil)}
	g := buildFlowGraph(flow)
	warns := lintChainDBWriteBeforeValidation("Svc.Op", g)
	if len(warns) != 0 {
		t.Errorf("lint should be silent when no validation step exists, got %d", len(warns))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Proof report
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildProofReport_ProvedFlow(t *testing.T) {
	t.Parallel()
	services := []normalizer.Service{{
		Name: "User",
		Methods: []normalizer.Method{{
			Name: "Create",
			Flow: []normalizer.FlowStep{
				makeStep("logic.Validate", nil),
				makeStep("repo.Save", nil),
			},
		}},
	}}
	endpoints := []normalizer.Endpoint{{
		ServiceName: "User",
		RPC:         "Create",
		Method:      "POST",
		AuthType:    "jwt",
	}}
	report := BuildProofReport(services, endpoints)
	if report.Schema != "ang/proof/v1" {
		t.Fatalf("unexpected schema: %s", report.Schema)
	}
	if len(report.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(report.Operations))
	}
	op := report.Operations[0]
	if op.Operation != "User.Create" {
		t.Errorf("unexpected operation: %s", op.Operation)
	}
	// auth_before_write should be proved (endpoint has auth)
	for _, p := range op.Properties {
		if p.Property == "auth_before_write" && p.Result != ProofProved {
			t.Errorf("auth_before_write expected proved, got %s (violations: %v)", p.Result, p.Violations)
		}
		if p.Property == "validation_before_write" && p.Result != ProofProved {
			t.Errorf("validation_before_write expected proved, got %s", p.Result)
		}
	}
}

func TestBuildProofReport_ViolatedFlow(t *testing.T) {
	t.Parallel()
	services := []normalizer.Service{{
		Name: "Order",
		Methods: []normalizer.Method{{
			Name: "Create",
			Flow: []normalizer.FlowStep{
				makeStep("repo.Save", nil),     // write before validate
				makeStep("event.Publish", nil), // unsafe publish
			},
		}},
	}}
	endpoints := []normalizer.Endpoint{{
		ServiceName: "Order",
		RPC:         "Create",
		Method:      "POST",
		// No auth
	}}
	report := BuildProofReport(services, endpoints)
	if len(report.Operations) != 1 {
		t.Fatalf("expected 1 operation")
	}
	op := report.Operations[0]
	hasViolation := false
	for _, p := range op.Properties {
		if p.Result == ProofViolated {
			hasViolation = true
		}
	}
	if !hasViolation {
		t.Error("expected at least one violated property for unguarded write + unsafe publish")
	}
}

func TestBuildProofReport_EventAtomicityViolated(t *testing.T) {
	t.Parallel()
	services := []normalizer.Service{{
		Name: "Blog",
		Methods: []normalizer.Method{{
			Name: "CreatePost",
			Flow: []normalizer.FlowStep{
				makeStep("repo.Save", nil),
				makeStep("event.Publish", nil), // should use event.Outbox
			},
		}},
	}}
	report := BuildProofReport(services, nil)
	if len(report.Operations) == 0 {
		t.Fatal("expected proof report")
	}
	for _, p := range report.Operations[0].Properties {
		if p.Property == "event_atomicity" {
			if p.Result != ProofViolated {
				t.Errorf("event_atomicity: expected violated, got %s", p.Result)
			}
			return
		}
	}
	t.Error("event_atomicity property not found in report")
}

func TestBuildProofReport_EventAtomicityProved_WithOutbox(t *testing.T) {
	t.Parallel()
	inner := []normalizer.FlowStep{
		makeStep("repo.Save", nil),
		makeStep("event.Outbox", nil),
	}
	services := []normalizer.Service{{
		Name: "Blog",
		Methods: []normalizer.Method{{
			Name: "CreatePost",
			Flow: []normalizer.FlowStep{
				makeStep("tx.Block", map[string]any{"_do": inner}),
			},
		}},
	}}
	report := BuildProofReport(services, nil)
	if len(report.Operations) == 0 {
		t.Fatal("expected proof report")
	}
	for _, p := range report.Operations[0].Properties {
		if p.Property == "event_atomicity" {
			if p.Result != ProofProved {
				t.Errorf("event_atomicity: expected proved with outbox, got %s violations=%v", p.Result, p.Violations)
			}
			return
		}
	}
	t.Error("event_atomicity property not found")
}

// ─────────────────────────────────────────────────────────────────────────────
// hasControlParentOfKind
// ─────────────────────────────────────────────────────────────────────────────

func TestHasControlParentOfKind(t *testing.T) {
	t.Parallel()
	inner := []normalizer.FlowStep{makeStep("http.Request", nil)}
	flow := []normalizer.FlowStep{
		makeStep("flow.Try", map[string]any{"_do": inner}),
	}
	g := buildFlowGraph(flow)
	// Node 0: flow.Try, Node 1: http.Request
	if len(g.Nodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(g.Nodes))
	}
	// http.Request (index 1) should have flow.Try as control parent
	if !hasControlParentOfKind(g, 1, "flow.Try") {
		t.Error("expected flow.Try to be control parent of http.Request")
	}
	if hasControlParentOfKind(g, 1, "flow.Saga") {
		t.Error("flow.Saga should NOT be a parent")
	}
}
