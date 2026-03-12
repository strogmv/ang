package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestEnsureRuntimeConfigFields_IncludesPostgresPoolSettings(t *testing.T) {
	t.Parallel()

	cfg := ensureRuntimeConfigFields(&normalizer.ConfigDef{})
	fields := map[string]normalizer.Field{}
	for _, f := range cfg.Fields {
		fields[f.Name] = f
	}

	cases := []struct {
		name string
		typ  string
		env  string
		def  string
	}{
		{name: "PGMaxConns", typ: "int", env: "PG_MAX_CONNS", def: "25"},
		{name: "PGMinConns", typ: "int", env: "PG_MIN_CONNS", def: "5"},
		{name: "PGMaxConnLifetime", typ: "string", env: "PG_MAX_CONN_LIFETIME", def: "1h"},
		{name: "PGMaxConnIdleTime", typ: "string", env: "PG_MAX_CONN_IDLE_TIME", def: "30m"},
	}
	for _, tc := range cases {
		got, ok := fields[tc.name]
		if !ok {
			t.Fatalf("expected config field %q to be injected", tc.name)
		}
		if got.Type != tc.typ || got.EnvVar != tc.env || got.Default != tc.def {
			t.Fatalf(
				"unexpected field %s: type=%q env=%q default=%q",
				tc.name, got.Type, got.EnvVar, got.Default,
			)
		}
	}
}

func TestEnsureRuntimeConfigFields_JWTPublicKeyIsOptional(t *testing.T) {
	t.Parallel()

	cfg := ensureRuntimeConfigFields(&normalizer.ConfigDef{})
	for _, f := range cfg.Fields {
		if f.Name == "JWTPublicKey" {
			if !f.IsOptional {
				t.Fatalf("JWTPublicKey must be optional")
			}
			return
		}
	}
	t.Fatalf("JWTPublicKey field was not injected")
}

func TestEnsureRuntimeConfigFields_EmailProviderDefaultsToNoop(t *testing.T) {
	t.Parallel()

	cfg := ensureRuntimeConfigFields(&normalizer.ConfigDef{})
	fields := map[string]normalizer.Field{}
	for _, f := range cfg.Fields {
		fields[f.Name] = f
	}

	if got := fields["EmailProvider"]; got.Default != "noop" || got.EnvVar != "EMAIL_PROVIDER" {
		t.Fatalf("unexpected EmailProvider field: %+v", got)
	}
	for _, name := range []string{"SMTPHost", "SMTPUser", "SMTPPass", "SMTPFrom"} {
		f, ok := fields[name]
		if !ok {
			t.Fatalf("missing %s field", name)
		}
		if !f.IsOptional {
			t.Fatalf("%s should be optional", name)
		}
	}
}

func TestEmitConfig_WritesBackendEnvExample(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "sdk", "templates")
	if err := em.EmitConfig(&normalizer.ConfigDef{}); err != nil {
		t.Fatalf("EmitConfig: %v", err)
	}

	envPath := filepath.Join(tmp, ".env.example")
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "JWT_ALG=HS256") {
		t.Fatalf(".env.example missing JWT_ALG default: %s", content)
	}
	if !strings.Contains(content, "JWT_PUBLIC_KEY=") {
		t.Fatalf(".env.example missing JWT_PUBLIC_KEY key: %s", content)
	}
}

func TestEnsureRuntimeConfigFields_DatabaseURLMatchesComposeDefaults(t *testing.T) {
	t.Parallel()

	cfg := ensureRuntimeConfigFields(&normalizer.ConfigDef{})
	fields := map[string]normalizer.Field{}
	for _, f := range cfg.Fields {
		fields[f.Name] = f
	}

	got, ok := fields["DatabaseURL"]
	if !ok {
		t.Fatalf("expected DatabaseURL field to be injected")
	}
	want := "postgres://app:app@localhost:5439/app?sslmode=disable"
	if got.Default != want {
		t.Fatalf("unexpected DatabaseURL default: got %q want %q", got.Default, want)
	}
}
