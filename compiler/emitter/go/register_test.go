package goemitter

import (
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/generator"
)

func TestRegisterGoSteps_Smoke(t *testing.T) {
	reg := generator.NewStepRegistry()
	Register(reg, RegisterInput{})
	steps := reg.Steps()
	if len(steps) < 10 {
		t.Fatalf("expected many go steps, got %d", len(steps))
	}
	mainStep := findStep(steps, "Server Main")
	if mainStep == nil {
		t.Fatalf("Server Main step not found")
	}
	if !hasCapability(mainStep.Requires, compiler.CapabilityProfileGoLegacy) {
		t.Fatalf("Server Main must require profile_go_legacy capability")
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
