package emitter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestEmitPythonProductionScaffolds(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")

	if err := em.EmitPythonConfig(&normalizer.ConfigDef{
		Fields: []normalizer.Field{
			{Name: "AppName", Type: "string", Default: "ang"},
			{Name: "Port", Type: "int"},
		},
	}); err != nil {
		t.Fatalf("emit python config: %v", err)
	}
	if err := em.EmitPythonRBAC(&normalizer.RBACDef{
		Roles: map[string][]string{"admin": []string{"user.read", "user.write"}},
	}); err != nil {
		t.Fatalf("emit python rbac: %v", err)
	}
	if err := em.EmitPythonAuthStores(&normalizer.AuthDef{RefreshStore: "postgres"}); err != nil {
		t.Fatalf("emit python auth stores: %v", err)
	}

	paths := []string{
		filepath.Join(tmp, "app", "config.py"),
		filepath.Join(tmp, "app", "security", "__init__.py"),
		filepath.Join(tmp, "app", "security", "rbac.py"),
		filepath.Join(tmp, "app", "security", "auth.py"),
		filepath.Join(tmp, "app", "security", "refresh_store.py"),
		filepath.Join(tmp, "app", "security", "refresh_store_memory.py"),
		filepath.Join(tmp, "app", "security", "refresh_store_postgres.py"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file %s: %v", p, err)
		}
	}
}
