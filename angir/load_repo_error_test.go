package angir

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Item 1 of docs/tech-debt-backlog.md: `\s` in a finder's sql is not a CUE
// escape; the file used to be skipped and its finders dropped without an error.
func TestLoadFailsOnInvalidEscapeInFinderSQL(t *testing.T) {
	dst := t.TempDir()
	src := filepath.Join("testdata", "minimal")
	if err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	}); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	bad := "package repo\n\nRepositories: Order: finders: [{\n\tname: \"Search\"\n\tsql: \"\"\"\n\t\tSELECT id FROM orders WHERE note ~ '\\s+'\n\t\t\"\"\"\n}]\n"
	if err := os.WriteFile(filepath.Join(dst, "cue", "repo", "search.cue"), []byte(bad), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Load(dst)
	if err == nil {
		t.Fatalf("expected an error for the invalid escape")
	}
	for _, want := range []string{"search.cue:6:", "unknown escape sequence", `finder "Search"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
}
