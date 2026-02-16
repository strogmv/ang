package python

import (
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter/contracts"
	"github.com/strogmv/ang/compiler/generator"
)

func TestInfraPythonStepsContainAuthStores(t *testing.T) {
	t.Parallel()

	def, ok := infraPythonStepDefs[contracts.InfraKeyAuth]
	if !ok {
		t.Fatalf("expected auth runner in python infra runners")
	}
	if def.Runner == nil {
		t.Fatalf("expected non-nil auth runner")
	}

	steps := contracts.InfraStepsForValuesPython(map[string]any{
		contracts.InfraKeyAuth: &contracts.AuthDef{},
	})
	if len(steps) != 1 {
		t.Fatalf("expected one infra step from registry, got %d", len(steps))
	}
	if steps[0].Name != "Python Auth Stores" {
		t.Fatalf("unexpected step name %q", steps[0].Name)
	}
	caps := toCapabilities(steps[0].Requires)
	if len(caps) != 2 || caps[0] != compiler.CapabilityProfilePythonFastAPI || caps[1] != compiler.CapabilityAuth {
		t.Fatalf("unexpected capabilities: %+v", caps)
	}
}

func TestRegisterInfraPythonStepsUsesLocalManifest(t *testing.T) {
	t.Parallel()

	reg := generator.NewStepRegistry()
	registerInfraPythonSteps(reg, RegisterInput{
		InfraValues: map[string]any{
			contracts.InfraKeyAuth: &contracts.AuthDef{},
		},
	})
	steps := reg.Steps()
	if len(steps) != 1 {
		t.Fatalf("expected 1 registered python infra step, got %d", len(steps))
	}
}
