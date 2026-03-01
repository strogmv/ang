package compiler

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestResolveTargetCapabilities_Go(t *testing.T) {
	caps, err := ResolveTargetCapabilities(normalizer.TargetDef{
		Lang:      "go",
		Framework: "chi",
		DB:        "postgres",
		Queue:     "nats",
	})
	if err != nil {
		t.Fatalf("resolve go caps: %v", err)
	}
	if !caps.HasAll(
		CapabilityProfileGoLegacy,
		CapabilityHTTP,
		CapabilitySQLRepo,
		CapabilityWS,
		CapabilityEvents,
		CapabilityAuth,
	) {
		t.Fatalf("missing expected go capabilities")
	}
}

func TestResolveTargetCapabilities_PythonFastAPI(t *testing.T) {
	caps, err := ResolveTargetCapabilities(normalizer.TargetDef{
		Lang:      "python",
		Framework: "fastapi",
		DB:        "postgres",
		Queue:     "nats",
	})
	if err != nil {
		t.Fatalf("resolve python caps: %v", err)
	}
	if !caps.HasAll(
		CapabilityProfilePythonFastAPI,
		CapabilityHTTP,
		CapabilitySQLRepo,
		CapabilityEvents,
		CapabilityAuth,
	) {
		t.Fatalf("missing expected python capabilities")
	}
	if caps.Has(CapabilityWS) {
		t.Fatalf("python fastapi profile should not claim ws capability by default")
	}
}

func TestResolveTargetCapabilities_GoGRPC(t *testing.T) {
	caps, err := ResolveTargetCapabilities(normalizer.TargetDef{
		Lang:      "go",
		Framework: "grpc",
		DB:        "postgres",
		Queue:     "nats",
	})
	if err != nil {
		t.Fatalf("resolve go grpc caps: %v", err)
	}
	if !caps.HasAll(
		CapabilityProfileGoLegacy,
		CapabilityGRPC,
		CapabilitySQLRepo,
		CapabilityWS,
		CapabilityEvents,
		CapabilityAuth,
	) {
		t.Fatalf("missing expected go grpc capabilities")
	}
}
