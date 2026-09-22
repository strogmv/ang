package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitTracing_UsesOnlyExplicitSecureEndpoint(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	if err := em.EmitTracing(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "internal", "pkg", "tracing", "tracing.go"))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(data)
	for _, want := range []string{
		"func Init(serviceName, endpoint string)",
		"if endpoint == \"\"",
		"otlptracehttp.WithEndpointURL(endpoint)",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated tracing.go is missing %q", want)
		}
	}
	if strings.Contains(generated, "WithInsecure") {
		t.Fatal("generated tracing.go must not force plaintext OTLP transport")
	}
}

func TestMainServerTracingOnlyInitializesWithConfiguredEndpoint(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "templates", "main_server", "root.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{
		"tracing.Init(\"ang-service\", cfg.OTELExporterOTLPTracesEndpoint)",
		"else if tp != nil",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("root.tmpl is missing %q", want)
		}
	}
}
