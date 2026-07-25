package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_AppliesURLValidationToStringArrayItems(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	entities := []ir.Entity{{
		Name: "MediaRequest",
		Fields: []ir.Field{{
			Name:        "mediaUrls",
			Type:        ir.TypeRef{Kind: ir.KindList, ItemType: &ir.TypeRef{Kind: ir.KindString}},
			Optional:    true,
			ValidateTag: "url",
		}},
	}}

	if err := em.EmitFrontendSDK(entities, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	text, err := os.ReadFile(filepath.Join(tmp, "schemas", "index.ts"))
	if err != nil {
		t.Fatalf("read schemas: %v", err)
	}
	out := string(text)
	if !strings.Contains(out, "mediaUrls: z.array(z.string().url()).optional(),") {
		t.Fatalf("expected URL validation on each string array item, got:\n%s", out)
	}
	if strings.Contains(out, "z.array(z.string()).url()") {
		t.Fatalf("must not call .url() on a Zod array, got:\n%s", out)
	}
}
