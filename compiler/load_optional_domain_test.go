package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/angir/parser"
)

// One file that does not parse fails the whole folder. It used to be skipped
// with a note on stderr and its declarations vanished from the generated code
// (item 1 of docs/tech-debt-backlog.md).
func TestLoadOptionalDomain_OneFileThatDoesNotParseFails(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "cue", "api")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	valid := "package api\nGood: \"ok\"\n"
	invalid := "package api\nBroken: {\n"
	if err := os.WriteFile(filepath.Join(dir, "valid.cue"), []byte(valid), 0o644); err != nil {
		t.Fatalf("write valid: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.cue"), []byte(invalid), 0o644); err != nil {
		t.Fatalf("write broken: %v", err)
	}

	p := parser.New()
	_, ok, err := LoadOptionalDomain(p, dir)
	if err == nil {
		t.Fatalf("expected an error for broken.cue")
	}
	if ok {
		t.Fatalf("expected ok=false on error")
	}
	if msg := normalizer.DescribeCUELoadError(err).Error(); !strings.Contains(msg, "broken.cue:2:") {
		t.Fatalf("error does not name broken.cue:2: %s", msg)
	}
}

func TestLoadOptionalDomain_AllInvalidStillFails(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "cue", "api")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	invalid := "package api\nBroken: {\n"
	if err := os.WriteFile(filepath.Join(dir, "broken.cue"), []byte(invalid), 0o644); err != nil {
		t.Fatalf("write broken: %v", err)
	}

	p := parser.New()
	_, ok, err := LoadOptionalDomain(p, dir)
	if err == nil {
		t.Fatalf("expected error for all-invalid CUE files")
	}
	if ok {
		t.Fatalf("expected ok=false on error")
	}
}
