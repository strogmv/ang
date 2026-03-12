package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestLoadInfraBundle_MergesCueInfraAndCueEffects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cue/infra"), 0o755); err != nil {
		t.Fatalf("mkdir cue/infra: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cue/effects"), 0o755); err != nil {
		t.Fatalf("mkdir cue/effects: %v", err)
	}

	infra := `package infra

#Models: {
  Cheap: "gpt-5-nano"
  Smart: "gpt-4o"
}

Handlers: {
  db: {driver: "postgres"}
}

TestHandlers: {
  db: {driver: "stub"}
}

Middleware: {
  db: [{type: "trace", level: "debug"}]
}
`
	effects := `package effects

Handlers: {
  events: {driver: "nats"}
  state:  {driver: "redis"}
}

TestHandlers: {
  events: {driver: "memory"}
}

Middleware: {
  events: [{type: "metrics"}]
  state:  [{type: "cache", ttl: "30s"}]
}
`
	if err := os.WriteFile(filepath.Join(root, "cue/infra", "infra.cue"), []byte(infra), 0o644); err != nil {
		t.Fatalf("write infra.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "cue/effects", "effects.cue"), []byte(effects), 0o644); err != nil {
		t.Fatalf("write effects.cue: %v", err)
	}

	bundle, err := LoadInfraBundle(root)
	if err != nil {
		t.Fatalf("LoadInfraBundle failed: %v", err)
	}
	if !bundle.Has || !bundle.InfraValueExists || !bundle.EffectsExists {
		t.Fatalf("expected both cue/infra and cue/effects to be loaded: %+v", bundle)
	}
	if bundle.Models == nil || bundle.Models.Aliases["Cheap"] != "gpt-5-nano" {
		t.Fatalf("expected models registry from cue/infra, got %+v", bundle.Models)
	}

	handlers := normalizer.InfraEffectHandlers(bundle.Values)
	if handlers == nil {
		t.Fatalf("expected handlers to be present")
	}
	if handlers.Bindings["db"].Driver != "postgres" {
		t.Fatalf("expected db handler from cue/infra, got %+v", handlers.Bindings["db"])
	}
	if handlers.Bindings["events"].Driver != "nats" {
		t.Fatalf("expected events handler from cue/effects, got %+v", handlers.Bindings["events"])
	}

	testHandlers := normalizer.InfraEffectTestHandlers(bundle.Values)
	if testHandlers == nil || testHandlers.Bindings["events"].Driver != "memory" {
		t.Fatalf("expected test events handler from cue/effects, got %+v", testHandlers)
	}

	middleware := normalizer.InfraEffectMiddleware(bundle.Values)
	if middleware == nil {
		t.Fatalf("expected middleware to be present")
	}
	if len(middleware.Chains["db"]) != 1 || middleware.Chains["db"][0].Type != "trace" {
		t.Fatalf("expected db middleware from cue/infra, got %+v", middleware.Chains["db"])
	}
	if len(middleware.Chains["state"]) != 1 || middleware.Chains["state"][0].Type != "cache" {
		t.Fatalf("expected state middleware from cue/effects, got %+v", middleware.Chains["state"])
	}
}
