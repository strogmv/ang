package compiler

import (
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
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

func TestResolveTargetCapabilities_RejectsOtherLanguages(t *testing.T) {
	for _, lang := range []string{"python", "rust", "typescript"} {
		if _, err := ResolveTargetCapabilities(normalizer.TargetDef{Lang: lang, Framework: "chi", DB: "postgres"}); err == nil {
			t.Fatalf("lang %q: expected an unsupported target error", lang)
		}
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
