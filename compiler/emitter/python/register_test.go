package python

import (
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

type emitterStub struct{}

func (emitterStub) EmitPythonConfig(cfg *normalizer.ConfigDef) error { return nil }
func (emitterStub) EmitPythonRBAC(rbac *normalizer.RBACDef) error    { return nil }
func (emitterStub) EmitPythonAuthStores(auth *normalizer.AuthDef) error {
	return nil
}
func (emitterStub) EmitPythonFastAPIBackendFromIR(schema *ir.Schema, fallbackVersion string) error {
	return nil
}

func TestRegisterPythonSteps_Smoke(t *testing.T) {
	reg := generator.NewStepRegistry()
	Register(reg, RegisterInput{
		Em: emitterStub{},
		InfraValues: map[string]any{
			normalizer.InfraKeyAuth: &normalizer.AuthDef{},
		},
	})
	steps := reg.Steps()
	if len(steps) < 4 {
		t.Fatalf("expected at least 4 python steps, got %d", len(steps))
	}
	fastAPI := findStep(steps, "Python FastAPI Backend")
	if fastAPI == nil {
		t.Fatalf("Python FastAPI Backend step not found")
	}
	if !hasCapability(fastAPI.Requires, compiler.CapabilityProfilePythonFastAPI) {
		t.Fatalf("fastapi step must require profile_python_fastapi capability")
	}
}

func findStep(steps []generator.Step, name string) *generator.Step {
	for i := range steps {
		if steps[i].Name == name {
			return &steps[i]
		}
	}
	return nil
}

func hasCapability(caps []compiler.Capability, cap compiler.Capability) bool {
	for _, c := range caps {
		if c == cap {
			return true
		}
	}
	return false
}
