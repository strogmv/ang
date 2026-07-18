package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
	"gopkg.in/yaml.v3"
)

func TestEmitOpenAPICombinesMethodsForSamePath(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "api", "openapi.yaml")
	emitter := New(root, root, "templates")
	endpoints := []normalizer.Endpoint{
		{Method: "GET", Path: "/api/items/{id}", RPC: "GetItem"},
		{Method: "DELETE", Path: "/api/items/{id}", RPC: "DeleteItem"},
	}
	if err := emitter.EmitOpenAPIFromNormalizerTypes(endpoints, nil, nil, nil, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("generated OpenAPI is invalid YAML: %v", err)
	}
	if count := strings.Count(string(data), "  /api/items/{id}:\n"); count != 1 {
		t.Fatalf("path emitted %d times, want 1:\n%s", count, data)
	}
	for _, method := range []string{"    get:", "    delete:"} {
		if !strings.Contains(string(data), method) {
			t.Fatalf("generated OpenAPI missing %q:\n%s", method, data)
		}
	}
}
