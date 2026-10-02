package emitter

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// nestedToursService is what the normalizer makes of
//
//	output: {
//	    settings: {remind: bool}
//	    pages: [...{pageKey: string, labels: [string]: string,
//	        steps: [...{anchor: string, hints: [...{text: string}]}]}]
//	}
func nestedToursService() normalizer.Service {
	hints := normalizer.Field{Name: "hints", Type: "[]GetToursResponsePagesItemStepsItemHintsItem", IsList: true,
		ItemTypeName: "GetToursResponsePagesItemStepsItemHintsItem",
		ItemFields:   []normalizer.Field{{Name: "text", Type: "string"}}}
	steps := normalizer.Field{Name: "steps", Type: "[]GetToursResponsePagesItemStepsItem", IsList: true,
		ItemTypeName: "GetToursResponsePagesItemStepsItem",
		ItemFields:   []normalizer.Field{{Name: "anchor", Type: "string"}, hints}}
	pages := normalizer.Field{Name: "pages", Type: "[]GetToursResponsePagesItem", IsList: true,
		ItemTypeName: "GetToursResponsePagesItem",
		ItemFields: []normalizer.Field{
			{Name: "pageKey", Type: "string"},
			{Name: "labels", Type: "map[string]string"},
			{Name: "flags", Type: "map[string]map[string]bool"},
			{Name: "grid", Type: "[][]GetToursResponsePagesItemGridItemItem", IsList: true,
				ItemTypeName: "GetToursResponsePagesItemGridItemItem",
				ItemFields:   []normalizer.Field{{Name: "x", Type: "int"}}},
			{Name: "byLang", Type: "map[string]GetToursResponsePagesItemByLangValue",
				ItemTypeName: "GetToursResponsePagesItemByLangValue",
				ItemFields:   []normalizer.Field{{Name: "title", Type: "string"}}},
			steps,
		}}
	settings := normalizer.Field{Name: "settings", Type: "GetToursResponseSettings",
		ItemTypeName: "GetToursResponseSettings",
		ItemFields:   []normalizer.Field{{Name: "remind", Type: "bool"}}}
	return normalizer.Service{
		Name: "Help",
		Methods: []normalizer.Method{{
			Name:   "GetTours",
			Input:  normalizer.Entity{Name: "GetToursRequest", Fields: []normalizer.Field{{Name: "lang", Type: "string"}}},
			Output: normalizer.Entity{Name: "GetToursResponse", Fields: []normalizer.Field{{Name: "version", Type: "string"}, settings, pages}},
		}},
	}
}

// Three levels of inline objects survive the IR round trip and every level
// is declared in Go, TypeScript, Zod and OpenAPI.
func TestNestedInlineTypesAreEmittedAtEveryLevel(t *testing.T) {
	svc := ir.ConvertService(nestedToursService())

	backDir := t.TempDir()
	frontDir := t.TempDir()
	em := New(backDir, frontDir, "templates")
	em.Version = "0.1.0"

	// Go port types.
	if err := em.EmitService([]ir.Service{svc}); err != nil {
		t.Fatalf("emit service: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(backDir, "internal", "port", "*.go"))
	var goSrc string
	for _, m := range matches {
		b, _ := os.ReadFile(m)
		if strings.Contains(string(b), "GetToursResponse") {
			goSrc = string(b)
			if _, err := parser.ParseFile(token.NewFileSet(), m, b, 0); err != nil {
				t.Fatalf("port file does not parse: %v\n%s", err, goSrc)
			}
		}
	}
	for _, want := range []string{
		"Settings GetToursResponseSettings",
		"Pages    []GetToursResponsePagesItem",
		"type GetToursResponseSettings struct",
		"type GetToursResponsePagesItem struct",
		"Labels  map[string]string",
		"Steps   []GetToursResponsePagesItemStepsItem",
		"type GetToursResponsePagesItemStepsItem struct",
		"Hints  []GetToursResponsePagesItemStepsItemHintsItem",
		"type GetToursResponsePagesItemStepsItemHintsItem struct",
		"ByLang  map[string]GetToursResponsePagesItemByLangValue",
		"type GetToursResponsePagesItemByLangValue struct",
		"Flags   map[string]map[string]bool",
		"Grid    [][]GetToursResponsePagesItemGridItemItem",
		"type GetToursResponsePagesItemGridItemItem struct",
	} {
		if !strings.Contains(goSrc, want) {
			t.Errorf("Go port lacks %q:\n%s", want, goSrc)
		}
	}
	if strings.Contains(goSrc, "[]string `json:\"steps\"`") {
		t.Errorf("steps degraded to []string:\n%s", goSrc)
	}

	// TypeScript types and Zod schemas.
	if err := em.EmitFrontendSDK(nil, []ir.Service{svc}, nil, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}
	types, err := os.ReadFile(filepath.Join(frontDir, "types", "index.ts"))
	if err != nil {
		t.Fatalf("read types: %v", err)
	}
	for _, want := range []string{
		"settings: GetToursResponseSettings;",
		"pages: GetToursResponsePagesItem[];",
		"labels: Record<string, string>;",
		"steps: GetToursResponsePagesItemStepsItem[];",
		"hints: GetToursResponsePagesItemStepsItemHintsItem[];",
		"export interface GetToursResponsePagesItemStepsItemHintsItem {",
		"byLang: Record<string, GetToursResponsePagesItemByLangValue>;",
		"flags: Record<string, Record<string, boolean>>;",
		"grid: GetToursResponsePagesItemGridItemItem[][];",
	} {
		if !strings.Contains(string(types), want) {
			t.Errorf("types/index.ts lacks %q", want)
		}
	}
	schemas, err := os.ReadFile(filepath.Join(frontDir, "schemas", "index.ts"))
	if err != nil {
		t.Fatalf("read schemas: %v", err)
	}
	for _, want := range []string{
		"z.lazy(() => GetToursResponseSettingsSchema)",
		"z.array(z.lazy(() => GetToursResponsePagesItemStepsItemHintsItemSchema))",
		"z.record(z.string(), z.string())",
		"z.record(z.string(), z.lazy(() => GetToursResponsePagesItemByLangValueSchema))",
		"z.record(z.string(), z.record(z.string(), z.boolean()))",
		"z.array(z.array(z.lazy(() => GetToursResponsePagesItemGridItemItemSchema)))",
	} {
		if !strings.Contains(string(schemas), want) {
			t.Errorf("schemas/index.ts lacks %q", want)
		}
	}

	// OpenAPI.
	openapiPath := filepath.Join(backDir, "openapi.yaml")
	endpoints := []normalizer.Endpoint{{Method: "GET", Path: "/api/help/tours", ServiceName: "Help", RPC: "GetTours"}}
	if err := em.EmitOpenAPIFromNormalizerTypes(endpoints, IRServicesToNormalizer([]ir.Service{svc}), nil, nil, openapiPath); err != nil {
		t.Fatalf("emit openapi: %v", err)
	}
	spec, err := os.ReadFile(openapiPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"$ref: '#/components/schemas/GetToursResponseSettings'",
		"$ref: '#/components/schemas/GetToursResponsePagesItemStepsItemHintsItem'",
		"    GetToursResponsePagesItemStepsItemHintsItem:",
		"additionalProperties:",
		"    GetToursResponsePagesItemByLangValue:",
		"    GetToursResponsePagesItemGridItemItem:",
		"          additionalProperties:\n            type: object\n            additionalProperties:\n              type: boolean",
	} {
		if !strings.Contains(string(spec), want) {
			t.Errorf("openapi lacks %q:\n%s", want, spec)
		}
	}
}
