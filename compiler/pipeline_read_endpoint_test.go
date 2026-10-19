package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler"
)

func TestPipelineReadsTheReadMarker(t *testing.T) {
	write := func(api string) (string, error) {
		root := t.TempDir()
		for rel, text := range map[string]string{
			"cue/domain/product.cue": "package domain\n\nProduct: {\n\tid: string\n}\n",
			"cue/api/catalog.cue":    "package api\n\n" + api,
		} {
			path := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, _, endpoints, _, _, _, _, _, _, err := compiler.RunPipeline(root)
		var marks []string
		for _, ep := range endpoints {
			if ep.Read {
				marks = append(marks, ep.RPC)
			}
			if ep.RPC == "SearchProducts" && len(ep.Invalidate) > 0 {
				t.Errorf("a read invalidates nothing, got %v", ep.Invalidate)
			}
		}
		return strings.Join(marks, ","), err
	}
	ops := `SearchProducts: {
	service: "catalog"
	input: {query: string}
	output: {total: int}
}
ListProducts: {
	service: "catalog"
	input: {}
	output: {total: int}
}
`
	got, err := write(ops + `
HTTP: SearchProducts: {method: "POST", path: "/products/search", read: true}
HTTP: ListProducts: {method: "GET", path: "/products"}
`)
	if err != nil || got != "SearchProducts" {
		t.Fatalf("read marks = %q, err %v", got, err)
	}
	if _, err := write(ops + `
HTTP: SearchProducts: {method: "WS", path: "/ws/search", read: true}
`); err == nil || !strings.Contains(err.Error(), "read: true is for HTTP endpoints") {
		t.Fatalf("read on WS must fail, got %v", err)
	}
}
