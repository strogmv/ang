package goemitter

import (
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/generator"
)

func TestRegisterGoSteps_Smoke(t *testing.T) {
	reg := generator.NewStepRegistry()
	Register(reg, RegisterInput{})
	if err := reg.Err(); err != nil {
		t.Fatalf("registry should be valid and deterministic, got error: %v", err)
	}
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

	serviceImplSteps := 0
	for _, step := range steps {
		if step.ArtifactKey == "go:service_impl" {
			serviceImplSteps++
		}
	}
	if serviceImplSteps != 1 {
		t.Fatalf("expected exactly one active service impl emitter path, got %d", serviceImplSteps)
	}

	grpcProtoStep := findStep(steps, "gRPC Proto")
	if grpcProtoStep == nil {
		t.Fatalf("gRPC Proto step not found")
	}
	if !hasCapability(grpcProtoStep.Requires, compiler.CapabilityGRPC) {
		t.Fatalf("gRPC Proto must require grpc capability")
	}
	if !hasCapability(grpcProtoStep.Requires, compiler.CapabilityProfileGoLegacy) {
		t.Fatalf("gRPC Proto must require profile_go_legacy capability")
	}

	grpcTransportStep := findStep(steps, "gRPC Transport")
	if grpcTransportStep == nil {
		t.Fatalf("gRPC Transport step not found")
	}
	if !hasCapability(grpcTransportStep.Requires, compiler.CapabilityGRPC) {
		t.Fatalf("gRPC Transport must require grpc capability")
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
