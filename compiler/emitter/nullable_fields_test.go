package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

func nullableMeta() map[string]any { return map[string]any{normalizer.FieldMetaNullable: true} }

func nullableSchema() *ir.Schema {
	str := ir.TypeRef{Kind: ir.KindString}
	boolean := ir.TypeRef{Kind: ir.KindBool}
	integer := ir.TypeRef{Kind: ir.KindInt}
	return &ir.Schema{
		Services: []ir.Service{{
			Name: "Tenders",
			Methods: []ir.Method{
				{
					Name: "ListTenders",
					Input: &ir.Entity{Name: "ListTendersRequest", Fields: []ir.Field{
						{Name: "favorite", Type: boolean, Optional: true, Metadata: nullableMeta()},
						{Name: "minLots", Type: integer, Optional: true, Metadata: nullableMeta()},
						{Name: "archived", Type: boolean, Optional: true},
					}},
					Output: &ir.Entity{Name: "ListTendersResponse", Fields: []ir.Field{{Name: "total", Type: integer}}},
				},
				{
					Name: "UpdateTender",
					Input: &ir.Entity{Name: "UpdateTenderRequest", Fields: []ir.Field{
						{Name: "id", Type: str},
						{Name: "openEntry", Type: boolean, Optional: true, Metadata: nullableMeta()},
						{Name: "note", Type: str, Optional: true, Metadata: nullableMeta()},
					}},
					Output: &ir.Entity{Name: "UpdateTenderResponse", Fields: []ir.Field{{Name: "ok", Type: boolean}}},
				},
			},
		}},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/tenders", Service: "Tenders", RPC: "ListTenders"},
			{Method: "PATCH", Path: "/tenders/{id}", Service: "Tenders", RPC: "UpdateTender"},
		},
	}
}

// @nullable() gives an optional scalar three states: the port field is a
// pointer, query parsing stores its address, TypeScript and Zod accept null,
// OpenAPI says nullable (backlog 47). Fields without it keep their types.
func TestNullableFieldsArePointersEverywhere(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.GoModule = "example.com/shop"
	schema := nullableSchema()
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitHTTPFromIR(schema, nil); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitOpenAPI(schema.Endpoints, schema.Services, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitFrontendSDK(nil, schema.Services, schema.Endpoints, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	read := func(rel ...string) string {
		data, err := os.ReadFile(filepath.Join(rel...))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	normalize := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	checks := map[string][]string{
		filepath.Join(root, "internal", "port", "tenders.go"): {
			"Favorite *bool `json:\"favorite\"`",
			"MinLots *int `json:\"minLots\"`",
			"Archived bool `json:\"archived\"`",
			"OpenEntry *bool `json:\"openEntry\"`",
			"Note *string `json:\"note\"`",
		},
		filepath.Join(root, "internal", "transport", "http", "tenders.go"): {
			"req.Favorite = &b",
			"req.MinLots = &i",
			"req.Archived = b",
		},
		filepath.Join(root, "api", "openapi.yaml"): {"nullable: true"},
		filepath.Join(root, "sdk", "types", "index.ts"): {
			"favorite?: boolean | null;",
			"openEntry?: boolean | null;",
			"archived?: boolean;",
		},
		filepath.Join(root, "sdk", "schemas", "index.ts"): {
			"openEntry: z.boolean().nullable().optional(),",
			"archived: z.boolean().optional(),",
		},
	}
	for path, wants := range checks {
		src := normalize(read(path))
		for _, want := range wants {
			if !strings.Contains(src, normalize(want)) {
				t.Errorf("%s lacks %q:\n%s", filepath.Base(path), want, read(path))
			}
		}
	}
}
