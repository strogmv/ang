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
		"authCfg.AuthSessionMaxTTL",
		// Only real activity moves the idle deadline.
		"func requestCountsAsActivity",
		`sessionActivityHeader  = "X-Session-Activity"`,
		// Expiry is told apart from "never signed in", and a store outage is not a sign-out.
		`"SESSION_IDLE_TIMEOUT"`,
		`"SESSION_EXPIRED"`,
		`"SESSION_STORE_UNAVAILABLE"`,
		"writeSessionHeaders(w, session)",
		"func opaqueSessionAlive",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated common.go is missing %q", want)
		}
	}
}

func TestEnsureRuntimeConfigFields_AuthSessionMaxTTLDefaultsDisabled(t *testing.T) {
	t.Parallel()

	cfg := ensureRuntimeConfigFields(&normalizer.ConfigDef{})
	for _, field := range cfg.Fields {
		if field.Name == "AuthSessionMaxTTL" {
			if field.Type != "string" || field.EnvVar != "AUTH_SESSION_MAX_TTL" || field.Default != "" {
				t.Fatalf("unexpected AuthSessionMaxTTL field: %+v", field)
			}
			return
		}
	}
	t.Fatal("AuthSessionMaxTTL field was not injected")
}
