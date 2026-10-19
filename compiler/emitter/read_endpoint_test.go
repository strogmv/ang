package emitter

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

// A POST marked read: true is a query for the SDK: useQuery, a query key and
// options, no mutation hook (backlog 49). The same POST without the marker
// stays a mutation.
func TestReadPostEndpointsGetQueryHooks(t *testing.T) {
	t.Parallel()
	str := ir.TypeRef{Kind: ir.KindString}
	services := []ir.Service{{
		Name: "Catalog",
		Methods: []ir.Method{
			{
				Name:   "SearchProducts",
				Input:  &ir.Entity{Name: "SearchProductsRequest", Fields: []ir.Field{{Name: "query", Type: str}}},
				Output: &ir.Entity{Name: "SearchProductsResponse", Fields: []ir.Field{{Name: "total", Type: ir.TypeRef{Kind: ir.KindInt}}}},
			},
			{
				Name:   "ImportProducts",
				Input:  &ir.Entity{Name: "ImportProductsRequest", Fields: []ir.Field{{Name: "file", Type: str}}},
				Output: &ir.Entity{Name: "ImportProductsResponse", Fields: []ir.Field{{Name: "ok", Type: ir.TypeRef{Kind: ir.KindBool}}}},
			},
		},
	}}
	endpoints := []ir.Endpoint{
		{Method: "POST", Path: "/catalog/search", Service: "Catalog", RPC: "SearchProducts", Read: true},
		{Method: "POST", Path: "/catalog/import", Service: "Catalog", RPC: "ImportProducts"},
	}
	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitOpenAPI(endpoints, services, nil, nil); err != nil {
		t.Fatal(err)
	}
	read := func(rel ...string) string {
		data, err := os.ReadFile(filepath.Join(append([]string{root}, rel...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	hooks := read("sdk", "hooks", "index.ts")
	searchHook := regexp.MustCompile(`export const useSearchProducts = [^\n]*`).FindString(hooks)
	if !strings.Contains(searchHook, "UseQueryOptions") {
		t.Fatalf("SearchProducts must be a query hook, got %q", searchHook)
	}
	if !strings.Contains(hooks, "useSearchProductsSuspense") {
		t.Fatal("a read gets the suspense hook too")
	}
	importHook := regexp.MustCompile(`export const useImportProducts = [^\n]*`).FindString(hooks)
	if strings.Contains(importHook, "UseQueryOptions") || importHook == "" {
		t.Fatalf("ImportProducts stays a mutation, got %q", importHook)
	}
	keys := read("sdk", "query-keys.ts")
	if !strings.Contains(keys, "searchProductsQueryKey") || strings.Contains(keys, "importProductsQueryKey") {
		t.Fatalf("query keys:\n%s", keys)
	}
	if spec := read("api", "openapi.yaml"); strings.Count(spec, "x-ang-read: true") != 1 {
		t.Fatalf("one operation is marked read in OpenAPI:\n%s", spec)
	}
}
