package emitter

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestHasMethodImplementation_CoversFlowImplManual(t *testing.T) {
	t.Run("flow", func(t *testing.T) {
		m := normalizer.Method{
			Name: "FlowMethod",
			Flow: []normalizer.FlowStep{{Action: "logic.Check"}},
		}
		if !hasMethodImplementation(m, map[string]bool{}) {
			t.Fatalf("expected flow method to be treated as implemented")
		}
	})

	t.Run("impl_code", func(t *testing.T) {
		m := normalizer.Method{
			Name: "ImplMethod",
			Impl: &normalizer.MethodImpl{Code: "return resp, nil"},
		}
		if !hasMethodImplementation(m, map[string]bool{}) {
			t.Fatalf("expected impl code method to be treated as implemented")
		}
	})

	t.Run("manual_override", func(t *testing.T) {
		m := normalizer.Method{Name: "ManualMethod"}
		if !hasMethodImplementation(m, map[string]bool{"ManualMethod": true}) {
			t.Fatalf("expected manual override method to be treated as implemented")
		}
	})

	t.Run("missing", func(t *testing.T) {
		m := normalizer.Method{Name: "MissingMethod"}
		if hasMethodImplementation(m, map[string]bool{}) {
			t.Fatalf("expected method without flow/impl/manual to be missing")
		}
	})
}

func TestAuditMissingImplementations_DeduplicatesAcrossEmitters(t *testing.T) {
	e := &Emitter{}
	svc := normalizer.Service{
		Name: "Auth",
		Methods: []normalizer.Method{
			{Name: "Missing", Source: "cue/api/auth.cue:10"},
			{Name: "FlowOk", Flow: []normalizer.FlowStep{{Action: "logic.Check"}}, Source: "cue/api/auth.cue:11"},
			{Name: "ImplOk", Impl: &normalizer.MethodImpl{Code: "return resp, nil"}, Source: "cue/api/auth.cue:12"},
		},
	}

	e.auditMissingImplementations(svc, map[string]bool{})
	e.auditMissingImplementations(svc, map[string]bool{}) // second pass simulates another emitter step

	if len(e.MissingImpls) != 1 {
		t.Fatalf("expected 1 unique missing impl, got %d (%+v)", len(e.MissingImpls), e.MissingImpls)
	}
	got := e.MissingImpls[0]
	if got.Service != "Auth" || got.Method != "Missing" {
		t.Fatalf("unexpected missing impl entry: %+v", got)
	}
}
