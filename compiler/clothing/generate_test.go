package clothing

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRenderGeneratedSDK(t *testing.T) {
	files, err := Render([]byte(`{"materials":{},"garments":{},"reactions":[],"wash_facilities":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module clothing-proof\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "./...", "-count=1")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated SDK failed: %v\n%s", err, out)
	}
}
func TestRenderRejectsInvalidCatalog(t *testing.T) {
	for _, data := range []string{`{`, `{"materials":{"bad":{"name":"Bad","washability":{"blood":1001}}}}`} {
		if _, err := Render([]byte(data)); err == nil {
			t.Fatal("bad catalog accepted")
		}
	}
}
