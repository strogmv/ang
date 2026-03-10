package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestEmitEffectRegistry(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/project"}

	entities := []normalizer.Entity{
		{
			Name: "Post",
			Fields: []normalizer.Field{
				{Name: "id", Type: "string", DB: normalizer.DBMeta{Type: "uuid"}},
			},
		},
	}
	services := []normalizer.Service{
		{
			Name: "Blog",
			Methods: []normalizer.Method{
				{
					Name: "CreatePost",
					Flow: []normalizer.FlowStep{
						{
							Action: "tx.Block",
							Args: map[string]any{
								"_do": []normalizer.FlowStep{
									{Action: "repo.Save", Args: map[string]any{"source": "Post"}},
									{Action: "event.Publish"},
									{Action: "state.Get"},
								},
							},
						},
					},
				},
			},
		},
	}
	ctx := em.AnalyzeContext(services, entities, nil)

	infraValues := map[string]any{
		normalizer.InfraKeyEffectHandlers: &normalizer.EffectHandlersDef{
			Bindings: map[string]normalizer.EffectHandlerBinding{
				"db":     {Kind: "db", Driver: "postgres"},
				"events": {Kind: "events", Driver: "nats"},
				"state":  {Kind: "state", Driver: "redis"},
			},
		},
		normalizer.InfraKeyEffectTestHandlers: &normalizer.EffectHandlersDef{
			Bindings: map[string]normalizer.EffectHandlerBinding{
				"db":     {Kind: "db", Driver: "stub"},
				"events": {Kind: "events", Driver: "memory"},
				"state":  {Kind: "state", Driver: "memory"},
			},
		},
		normalizer.InfraKeyEffectMiddleware: &normalizer.EffectMiddlewareCatalogDef{
			Chains: map[string][]normalizer.EffectMiddlewareDef{
				"events": {
					{Type: "retry", Attempts: 2, Backoff: "200ms"},
				},
			},
		},
	}

	if err := em.EmitEffectRegistry(ctx, infraValues); err != nil {
		t.Fatalf("EmitEffectRegistry failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "internal", "bootstrap", "effect_registry.gen.go"))
	if err != nil {
		t.Fatalf("read generated effect registry: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		"type EffectHandler struct",
		"type EffectProfile struct",
		"func (p EffectProfile) Handler(kind string) (EffectHandler, bool)",
		"func (p EffectProfile) Chain(kind string) []effectmw.EffectMiddleware",
		"type EffectRegistry struct",
		"RepoPost",
		"Runtime:",
		"Test:",
		"effectmw.WrapPublisher",
		"effectmw.WrapStateStore",
		"map[string][]effectmw.EffectMiddleware",
		`"events": {Kind: "events", Driver: "nats"`,
		`"events": {Kind: "events", Driver: "memory"`,
		`Type: "retry", Attempts: 2, Backoff: "200ms"`,
		"func NewEffectRegistry(",
		"func NewTestEffectRegistry(publisher port.Publisher, storage port.FileStorage, stateStore port.StateStore) *EffectRegistry",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated effect registry missing %q:\n%s", want, out)
		}
	}
}
