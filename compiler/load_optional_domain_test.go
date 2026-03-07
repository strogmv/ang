package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/parser"
)

func TestLoadOptionalDomain_PartialSyntaxFallback(t *testing.T) {
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
	val, ok, err := LoadOptionalDomain(p, dir)
	if err != nil {
		t.Fatalf("LoadOptionalDomain failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	got, err := val.LookupPath(cue.ParsePath("Good")).String()
	if err != nil || got != "ok" {
		t.Fatalf("expected Good=\"ok\", got %q err=%v", got, err)
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
