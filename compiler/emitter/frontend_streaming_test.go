package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_GeneratesStreamingHelpersForStreamingEndpoints(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	services := []ir.Service{
		{
			Name: "Sandbox",
			Methods: []ir.Method{
				{
					Name:        "StreamAIEdit",
					IsStreaming: true,
					Input:       &ir.Entity{Name: "StreamAIEditRequest"},
					Output:      &ir.Entity{Name: "StreamAIEditResponse"},
				},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:      "POST",
			Path:        "/api/projects/{projectID}/ai-edit/stream",
			Service:     "Sandbox",
			RPC:         "StreamAIEdit",
			IsStreaming: true,
		},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "endpoints", "sandbox.ts"))
	if err != nil {
		t.Fatalf("read split endpoints module: %v", err)
	}
	text := string(data)

	mustContain := []string{
		"import { useAuthStore } from '../auth-store';",
		"export async function* streamAIEditStream(",
		"AsyncGenerator<string, void, unknown>",
		"const token = useAuthStore.getState().token;",
		") => streamAIEditStream(params, init);",
			}
	for _, expected := range mustContain {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in split endpoints module, got:\n%s", expected, text)
		}
	}

	if strings.Contains(text, "validateResponse('StreamAIEditResponseSchema'") {
		t.Fatalf("streaming endpoint must not be generated as regular axios+validateResponse call, got:\n%s", text)
	}
}

func TestEmitFrontendSDK_OmitsStreamingHelpersWhenNoStreamingEndpoints(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	services := []ir.Service{
		{
			Name: "Tender",
			Methods: []ir.Method{
				{
					Name:   "ListTenders",
					Input:  &ir.Entity{Name: "ListTendersRequest"},
					Output: &ir.Entity{Name: "ListTendersResponse"},
				},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:  "GET",
			Path:    "/api/tenders",
			Service: "Tender",
			RPC:     "ListTenders",
		},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "endpoints", "tender.ts"))
	if err != nil {
		t.Fatalf("read split endpoints module: %v", err)
	}
	text := string(data)

	mustNotContain := []string{
		"import { useAuthStore } from '../auth-store';",
		"parseSSEEvent",
		"resolveStreamUrl",
		"AsyncGenerator<string, void, unknown>",
	}
	for _, unexpected := range mustNotContain {
		if strings.Contains(text, unexpected) {
			t.Fatalf("did not expect %q in split endpoints module, got:\n%s", unexpected, text)
		}
	}
}
