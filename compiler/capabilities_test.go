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

func TestResolveTargetCapabilities_RejectsStacksANGDoesNotGenerate(t *testing.T) {
	base := normalizer.TargetDef{Lang: "go", Framework: "chi", DB: "postgres", Cache: "redis", Queue: "nats", Storage: "s3"}
	for _, tc := range []struct {
		name string
		edit func(*normalizer.TargetDef)
	}{
		{"python", func(td *normalizer.TargetDef) { td.Lang = "python" }},
		{"rust", func(td *normalizer.TargetDef) { td.Lang = "rust" }},
		{"echo", func(td *normalizer.TargetDef) { td.Framework = "echo" }},
		{"mysql", func(td *normalizer.TargetDef) { td.DB = "mysql" }},
		{"memcached", func(td *normalizer.TargetDef) { td.Cache = "memcached" }},
		{"kafka", func(td *normalizer.TargetDef) { td.Queue = "kafka" }},
		{"gcs", func(td *normalizer.TargetDef) { td.Storage = "gcs" }},
	} {
		td := base
		tc.edit(&td)
		if _, err := ResolveTargetCapabilities(td); err == nil {
			t.Fatalf("%s: expected an unsupported target error", tc.name)
		}
	}
	if _, err := ResolveTargetCapabilities(base); err != nil {
		t.Fatalf("supported stack rejected: %v", err)
	}
	if _, err := ResolveTargetCapabilities(normalizer.TargetDef{Lang: "go", Queue: "none", Cache: "none", Storage: "none"}); err != nil {
		t.Fatalf("defaults with none rejected: %v", err)
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
