package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_MockHandlersUseExactResponseFieldNames(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	services := []ir.Service{
		{
			Name: "Attachments",
			Methods: []ir.Method{
				{
					Name:  "UploadAttachment",
					Input: &ir.Entity{Name: "UploadAttachmentRequest"},
					Output: &ir.Entity{
						Name: "UploadAttachmentResponse",
						Fields: []ir.Field{
							{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
							{Name: "url", Type: ir.TypeRef{Kind: ir.KindString}},
						},
					},
				},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{Method: "POST", Path: "/api/attachments", Service: "Attachments", RPC: "UploadAttachment"},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	text, err := os.ReadFile(filepath.Join(tmp, "mocks", "handlers.ts"))
	if err != nil {
		t.Fatalf("read mocks/handlers.ts: %v", err)
	}
	out := string(text)
	// Field names must use JSONName convention (camelCase, matching the TypeScript types).
	// "id" → "ID" (JSONName special-cases the standalone "id" acronym).
	// "url" → "url" (not ExportName "URL" — mock keys must match TS type keys, not Go field names).
	if !strings.Contains(out, `ID: "gen-id-123",`) {
		t.Fatalf("expected id field as 'ID' (JSONName acronym) in mocks/handlers.ts, got:\n%s", out)
	}
	if !strings.Contains(out, `url: "sample text",`) {
		t.Fatalf("expected url field as 'url' (JSONName) in mocks/handlers.ts, got:\n%s", out)
	}
	if strings.Contains(out, `URL: "sample text",`) {
		t.Fatalf("did not expect ExportName 'URL' in mocks/handlers.ts (must match TS type key), got:\n%s", out)
	}
}
