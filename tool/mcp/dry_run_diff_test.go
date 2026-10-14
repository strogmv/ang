package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDryRunPatchesOrdersTruncatesAndBudgets(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sdk/index.ts.patch", "--- a/sdk/index.ts\n+++ b/sdk/index.ts\n")
	write("internal/domain/user.go.patch", strings.Repeat("x", 50))
	write("zz/last.go.patch", "late")
	write("notes.txt", "not a patch")

	// Each of the first two patches is cut to 20 bytes, so 40 bytes exhaust the
	// budget and the third is listed without content.
	patches := readDryRunPatches(dir, 20, 40)
	if len(patches) != 3 {
		t.Fatalf("got %d patches, want 3: %v", len(patches), patches)
	}
	if patches[0]["path"] != "internal/domain/user.go" || patches[0]["truncated"] != true || len(patches[0]["patch"].(string)) != 20 {
		t.Fatalf("first patch = %v", patches[0])
	}
	if patches[1]["path"] != "sdk/index.ts" || patches[1]["patch"] == nil {
		t.Fatalf("second patch = %v", patches[1])
	}
	if patches[2]["path"] != "zz/last.go" || patches[2]["omitted"] == nil || patches[2]["patch"] != nil {
		t.Fatalf("patch past the budget = %v", patches[2])
	}
}
