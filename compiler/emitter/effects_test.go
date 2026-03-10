package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestEmitEffectsArtifacts(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/project"}

	portDir := filepath.Join(tmp, "internal", "port")
	if err := os.MkdirAll(portDir, 0o755); err != nil {
		t.Fatalf("mkdir port dir: %v", err)
	}
	portSrc := `package port

type Blog interface {
	Ping() error
}
`
	if err := os.WriteFile(filepath.Join(portDir, "blog.go"), []byte(portSrc), 0o644); err != nil {
		t.Fatalf("write port file: %v", err)
	}

	ctx := MainContext{
		Entities: []normalizer.Entity{{Name: "Project"}},
		EventPayloads: map[string]normalizer.Entity{
			"ProjectCreated": {Name: "ProjectCreated"},
		},
	}
	infraValues := map[string]any{
		normalizer.InfraKeyEffectHandlers: &normalizer.EffectHandlersDef{
			Bindings: map[string]normalizer.EffectHandlerBinding{
				"events":  {Kind: "events", Driver: "nats"},
				"storage": {Kind: "storage", Driver: "s3"},
				"state":   {Kind: "state", Driver: "redis"},
			},
		},
		normalizer.InfraKeyEffectTestHandlers: &normalizer.EffectHandlersDef{
			Bindings: map[string]normalizer.EffectHandlerBinding{
				"events":  {Kind: "events", Driver: "memory"},
				"storage": {Kind: "storage", Driver: "memory"},
				"state":   {Kind: "state", Driver: "memory"},
			},
		},
		normalizer.InfraKeyEffectMiddleware: &normalizer.EffectMiddlewareCatalogDef{
			Chains: map[string][]normalizer.EffectMiddlewareDef{
				"events":  {{Type: "retry", Attempts: 2, Backoff: "100ms"}},
				"storage": {{Type: "cache", TTL: "1m"}},
				"state":   {{Type: "trace"}},
			},
		},
	}

	if err := em.EmitEffectsArtifacts(ctx, infraValues); err != nil {
		t.Fatalf("EmitEffectsArtifacts failed: %v", err)
	}

	checks := []struct {
		path string
		want []string
	}{
		{
			path: filepath.Join(tmp, "internal", "bootstrap", "effect_registry.gen.go"),
			want: []string{"type EffectRegistry struct", `"events": {Kind: "events", Driver: "nats"`},
		},
		{
			path: filepath.Join(tmp, "internal", "adapter", "middleware", "effects.gen.go"),
			want: []string{"type EffectMiddleware struct", "func WrapPublisher(next port.Publisher, chain []EffectMiddleware) port.Publisher"},
		},
		{
			path: filepath.Join(tmp, "internal", "adapter", "mock", "blog.gen.go"),
			want: []string{"type MockBlog struct", "func NewBlog() *MockBlog"},
		},
	}
	for _, check := range checks {
		data, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read generated file %s: %v", check.path, err)
		}
		out := string(data)
		for _, want := range check.want {
			if !strings.Contains(out, want) {
				t.Fatalf("generated %s missing %q:\n%s", check.path, want, out)
			}
		}
	}
}
