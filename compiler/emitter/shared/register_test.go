package sharedsteps

import (
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/generator"
)

func TestRegisterSharedSteps_Smoke(t *testing.T) {
	reg := generator.NewStepRegistry()
	Register(reg, RegisterInput{})
	steps := reg.Steps()
	// Python SDK step removed from shared steps; now 3 steps: OpenAPI, AsyncAPI, System Manifest.
	if len(steps) < 3 {
		t.Fatalf("expected at least 3 shared steps, got %d", len(steps))
	}

	openAPI := findStep(steps, "OpenAPI")
	if openAPI == nil {
		t.Fatalf("OpenAPI step not found")
	}
	if !hasCapability(openAPI.Requires, compiler.CapabilityHTTP) {
		t.Fatalf("OpenAPI must require http capability")
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
