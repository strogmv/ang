package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitHTTP_StreamingEndpointUsesSSE(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	schema := &ir.Schema{
		Services: []ir.Service{
			{
				Name: "Sandbox",
				Methods: []ir.Method{
					{
						Name:        "StreamAIEdit",
						IsStreaming: true,
						Input:       &ir.Entity{Name: "StreamAIEditRequest", Fields: []ir.Field{{Name: "prompt", Type: ir.TypeRef{Kind: ir.KindString}}}},
						Output:      &ir.Entity{Name: "StreamAIEditResponse"},
					},
				},
			},
		},
		Endpoints: []ir.Endpoint{
			{
				Method:      "POST",
				Path:        "/projects/{projectID}/ai-edit/stream",
				Service:     "Sandbox",
				RPC:         "StreamAIEdit",
				IsStreaming: true,
			},
		},
	}

	em := New(root, filepath.Join(root, "sdk"), "templates")
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatalf("EmitServiceFromIR failed: %v", err)
	}
	if err := em.EmitHTTPFromIR(schema, nil); err != nil {
		t.Fatalf("EmitHTTPFromIR failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "internal", "transport", "http", "sandbox.go"))
	if err != nil {
		t.Fatalf("read generated HTTP file: %v", err)
	}
	src := string(data)
	for _, needle := range []string{
		`"text/event-stream"`,
		`chunks := make(chan string, 32)`,
		`svc.StreamAIEdit(r.Context(), req, chunks)`,
		`data: [DONE]`,
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("expected generated HTTP handler to contain %q, got:\n%s", needle, src)
		}
	}
}
