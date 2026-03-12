package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestEmitEffectMiddleware(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/project"}
	ctx := MainContext{
		EventPayloads: map[string]normalizer.Entity{
			"ProjectCreated": {Name: "ProjectCreated"},
		},
	}

	if err := em.EmitEffectMiddleware(ctx); err != nil {
		t.Fatalf("EmitEffectMiddleware failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "internal", "adapter", "middleware", "effects.gen.go"))
	if err != nil {
		t.Fatalf("read generated effect middleware: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		"type EffectMiddleware struct",
		"type effectPolicy struct",
		"var effectTracer = otel.Tracer(\"ang/effects\")",
		"var effectOpsTotal = promauto.NewCounterVec(",
		"func compilePolicy(chain []EffectMiddleware) effectPolicy",
		"func startEffectSpan(ctx context.Context, p effectPolicy, kind string, op string) (context.Context, oteltrace.Span)",
		"func cacheGet[T any](key string) (T, bool)",
		"func WrapPublisher(next port.Publisher, chain []EffectMiddleware) port.Publisher",
		"func WrapFileStorage(next port.FileStorage, chain []EffectMiddleware) port.FileStorage",
		"func WrapStateStore(next port.StateStore, chain []EffectMiddleware) port.StateStore",
		"func (m *publisherMiddleware) PublishProjectCreated(ctx context.Context, event domain.ProjectCreated) error",
		"func (m *publisherMiddleware) BroadcastProjectCreated(ctx context.Context, event domain.ProjectCreated) error",
		"func (m *publisherMiddleware) Wait(ctx context.Context, name string, match string) (any, error)",
		"func (m *fileStorageMiddleware) PresignGet(ctx context.Context, key string, expiresIn time.Duration) (string, error)",
		"func (m *stateStoreMiddleware) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated effect middleware missing %q:\n%s", want, out)
		}
	}
}
