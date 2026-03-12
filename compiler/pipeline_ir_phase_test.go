package compiler

import (
	"errors"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestRunIRPhase_OrdersServicesByDependencies(t *testing.T) {
	t.Parallel()

	out, err := RunIRPhase(IRPhaseInput{
		Normalized: NormalizePhaseOutput{
			Services: []normalizer.Service{
				{Name: "Billing", Uses: []string{"Auth"}},
				{Name: "Auth"},
			},
		},
	})
	if err != nil {
		t.Fatalf("RunIRPhase failed: %v", err)
	}
	if out.Schema == nil {
		t.Fatalf("expected non-nil IR schema")
	}
	if len(out.Schema.Services) != 2 {
		t.Fatalf("expected 2 services in schema, got %d", len(out.Schema.Services))
	}
	if out.Schema.Services[0].Name != "Auth" || out.Schema.Services[1].Name != "Billing" {
		t.Fatalf("unexpected service order: %#v", out.Schema.Services)
	}
}

func TestRunIRPhase_RejectsUnknownServiceDependency(t *testing.T) {
	t.Parallel()

	_, err := RunIRPhase(IRPhaseInput{
		Normalized: NormalizePhaseOutput{
			Services: []normalizer.Service{
				{Name: "Billing", Uses: []string{"Auth"}},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error for unknown dependency")
	}

	var cErr *ContractError
	if !errors.As(err, &cErr) {
		t.Fatalf("expected ContractError, got %T: %v", err, err)
	}
	if cErr.Stage != StageIR || cErr.Code != ErrCodeIRServiceDependencies {
		t.Fatalf("unexpected contract error: stage=%s code=%s err=%v", cErr.Stage, cErr.Code, cErr.Err)
	}
}

func TestCompileForEmit_EmptyProjectSmoke(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	out, err := CompileForEmit(base, PipelineOptions{}, CompileForEmitOptions{})
	if err != nil {
		t.Fatalf("CompileForEmit failed: %v", err)
	}
	if out.IR == nil {
		t.Fatalf("expected non-nil IR schema")
	}
}
