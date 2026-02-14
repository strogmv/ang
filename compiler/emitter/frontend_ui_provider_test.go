package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitBaseUIAutoFormLayer_UsesConfiguredUIProvider(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, tmp, "templates")
	em.UIProviderPath = "@/shared/ui/legacy-skin"

	if err := em.emitBaseUIAutoFormLayer(); err != nil {
		t.Fatalf("emit auto-form layer: %v", err)
	}

	autoFormPath := filepath.Join(tmp, "components", "ui", "auto-form", "AutoForm.tsx")
	data, err := os.ReadFile(autoFormPath)
	if err != nil {
		t.Fatalf("read generated AutoForm.tsx: %v", err)
	}

	got := string(data)
	if !strings.Contains(got, "from '@/shared/ui/legacy-skin'") {
		t.Fatalf("expected custom ui provider import, got:\n%s", got)
	}
}

func TestEmitBaseUIFormsProxyLayer_SkipsWhenCustomProvider(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, tmp, "templates")
	em.UIProviderPath = "@/shared/ui/legacy-skin"

	if err := em.emitBaseUIFormsProxyLayer(); err != nil {
		t.Fatalf("emit base ui forms proxy: %v", err)
	}

	path := filepath.Join(tmp, "components", "ui", "forms", "index.tsx")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no generated proxy layer for custom provider, path=%s err=%v", path, err)
	}
}

