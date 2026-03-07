package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzCompileForEmit_NoPanic(f *testing.F) {
	f.Add("package domain\nUser: { id: string }\n", "package api\nGetUser: { service: \"auth\" input: {} output: {} }\n")
	f.Add("package domain\nUser: {\n", "package api\n")
	f.Add("package domain\n", "package api\nBroken: {\n")

	f.Fuzz(func(t *testing.T, domainInput, apiInput string) {
		base := t.TempDir()
		mustMkdir(t, filepath.Join(base, "cue", "domain"))
		mustMkdir(t, filepath.Join(base, "cue", "api"))
		mustMkdir(t, filepath.Join(base, "cue", "architecture"))

		writeFile(t, filepath.Join(base, "cue", "domain", "fuzz_domain.cue"), limitSize(domainInput, 8192))
		writeFile(t, filepath.Join(base, "cue", "api", "fuzz_api.cue"), limitSize(apiInput, 8192))
		writeFile(t, filepath.Join(base, "cue", "architecture", "fuzz_arch.cue"), "package architecture\n")

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("CompileForEmit panicked: %v", r)
			}
		}()

		_, _ = CompileForEmit(base, PipelineOptions{}, CompileForEmitOptions{})
	})
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func limitSize(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
