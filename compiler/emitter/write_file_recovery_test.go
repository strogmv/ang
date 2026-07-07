package emitter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileIfChangedSelfHealsMissingAndDriftedArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "internal", "port", "mock.gen.go")
	desired := []byte("package port\n")

	// A missing generated file must be recreated even when all generator inputs
	// are otherwise unchanged.
	if err := WriteFileIfChanged(path, desired, 0o644); err != nil {
		t.Fatalf("recreate missing artifact: %v", err)
	}
	assertFileContent(t, path, desired)

	// Local drift must not be mistaken for an up-to-date cache hit.
	if err := os.WriteFile(path, []byte("drift"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileIfChanged(path, desired, 0o644); err != nil {
		t.Fatalf("repair drifted artifact: %v", err)
	}
	assertFileContent(t, path, desired)
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("content=%q want=%q", got, want)
	}
}
