package main

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzExtractViaTransform_NoPanic(f *testing.F) {
	f.Add("openapi: 3.0.0\npaths: {}\n")
	f.Add("openapi: 3.0.0\npaths:\n  /x:\n    get:\n      responses:\n        \"200\": {description: ok}\n")
	f.Fuzz(func(t *testing.T, spec string) {
		dir := t.TempDir()
		p := filepath.Join(dir, "openapi.yml")
		if err := os.WriteFile(p, []byte(spec), 0o644); err != nil {
			t.Fatal(err)
		}
		_, _ = extractViaTransform("openapi", p)
	})
}
