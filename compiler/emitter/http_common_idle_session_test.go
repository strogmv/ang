package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func TestEmitHTTPCommon_RenewsOpaqueSessionIdleDeadline(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	if err := em.EmitHTTPCommon(&normalizer.AuthDef{Mode: "opaque_session_cookie"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", "common.go"))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(data)
	for _, want := range []string{
		"authCfg.AuthSessionIdleTTL",
		"func touchOpaqueSession",
		"touchOpaqueSession(r.Context(), *payload)",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated common.go is missing %q", want)
		}
	}
}
