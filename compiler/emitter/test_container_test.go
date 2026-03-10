package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestEmitTestContainerFromIR(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/project"}
	schema := &ir.Schema{
		Entities: []ir.Entity{
			{Name: "User"},
		},
		Services: []ir.Service{
			{
				Name: "Auth",
				Methods: []ir.Method{
					{
						Name:   "Login",
						Input:  &ir.Entity{Name: "LoginRequest"},
						Output: &ir.Entity{Name: "LoginResponse"},
						Sources: []ir.Source{
							{Entity: "User"},
						},
					},
				},
			},
		},
	}
	ctx := em.AnalyzeContextFromIR(schema)
	auth := &normalizer.AuthDef{Service: "Auth", RefreshStore: "memory"}

	if err := em.EmitTestContainerFromIR(ctx, schema, auth, nil); err != nil {
		t.Fatalf("EmitTestContainerFromIR failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "internal", "bootstrap", "test_container.gen.go"))
	if err != nil {
		t.Fatalf("read generated test container: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		"type TestContainer struct",
		"UserRepository        *mock.MockUserRepository",
		"RefreshTokenStore     *mock.MockRefreshTokenStore",
		"func NewTestContainer(",
		"func NewTestContainerWith(opts ...TestOption) *TestContainer",
		"return NewTestContainer(opts...)",
		"c.Effects = NewTestEffectRegistry(",
		"service.NewAuthImpl(",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated test container missing %q:\n%s", want, out)
		}
	}
}
