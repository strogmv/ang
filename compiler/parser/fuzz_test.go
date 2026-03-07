package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzLoadDomain_NoPanicOnArbitraryCUE(f *testing.F) {
	f.Add("package x\nX: {action: \"repo.Get\"}\n")
	f.Add("package x\nX: {action: \"\"}\n")
	f.Add("package x\nX: {flow: []}\n")
	f.Add("package x\nX: {\n")

	f.Fuzz(func(t *testing.T, input string) {
		dir := t.TempDir()
		file := filepath.Join(dir, "fuzz_input.cue")
		if err := os.WriteFile(file, []byte(input), 0o644); err != nil {
			t.Fatalf("write fuzz input: %v", err)
		}

		p := New()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("LoadDomain panicked: %v", r)
			}
		}()
		_, _ = p.LoadDomain(dir)
	})
}
