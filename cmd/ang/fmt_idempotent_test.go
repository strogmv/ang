package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ang fmt followed by ang fmt --check must report nothing. On dealingi-back a
// single pass left 11 files for a second pass: cue fmt re-indented strings, and
// Go that had been skipped became formattable.
func TestFormatCueTreeIsIdempotent(t *testing.T) {
	root := t.TempDir()
	src := "package api\n\n" +
		"#Impls: {\n" +
		"  GetCapabilities: {\n" +
		"      flow: [\n" +
		"        {\n" +
		"          action: \"logic.Call\"\n" +
		"          func: \"\"\"\n" +
		"            (func(ctx context.Context) (string, error) {\n" +
		"              encode := func() (string, error) {\n" +
		"                return \"\", nil\n" +
		"              }\n" +
		"              \n" +
		"              if x:=1; x>0 { return encode() }\n" +
		"              return \"\", nil\n" +
		"            })\n" +
		"            \"\"\"\n" +
		"          output:    \"caps\"\n" +
		"        },\n" +
		"      ]\n" +
		"  }\n" +
		"}\n"
	path := filepath.Join(root, "capabilities.cue")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := formatCueTree(root, false); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	res, err := formatCueTree(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesChanged != 0 {
		second, _ := os.ReadFile(path)
		t.Fatalf("a second ang fmt would still change the file:\n--- after one run\n%s", first)
		_ = second
	}
}
