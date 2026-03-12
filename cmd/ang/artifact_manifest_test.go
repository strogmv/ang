package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang/compiler"
)

func TestArtifactManifestStableAcrossRuns(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mustWrite("internal/service/auth.go", "package service\n")
	mustWrite("api/openapi.yaml", "openapi: 3.1.0\n")
	mustWrite("sdk/endpoints.ts", "export const endpointMeta = {}\n")
	mustWrite("ang-manifest.json", "{}\n")

	targets := []artifactManifestTarget{
		{Mode: "in_place", Backend: root, Frontend: filepath.Join(root, "sdk")},
	}
	m1, err := buildArtifactHashManifest(root, targets, "2", "input-hash", "template-hash", compiler.BuildFingerprint())
	if err != nil {
		t.Fatalf("first manifest: %v", err)
	}
	m2, err := buildArtifactHashManifest(root, targets, "2", "input-hash", "template-hash", compiler.BuildFingerprint())
	if err != nil {
		t.Fatalf("second manifest: %v", err)
	}
	if len(m1.Artifacts) == 0 {
		t.Fatalf("expected non-empty artifact set")
	}
	if len(m1.Artifacts) != len(m2.Artifacts) {
		t.Fatalf("artifact count changed: %d vs %d", len(m1.Artifacts), len(m2.Artifacts))
	}
	for i := range m1.Artifacts {
		if m1.Artifacts[i] != m2.Artifacts[i] {
			t.Fatalf("manifest is unstable at index %d: %#v vs %#v", i, m1.Artifacts[i], m2.Artifacts[i])
		}
	}
}

func TestReadArtifactHashManifestRejectsStaleCompilerFingerprint(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".ang", "cache")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := `{
  "schemaVersion": "artifact-manifest/v2",
  "compilerVersion": "` + compiler.Version + `",
  "compilerFingerprint": "stale",
  "irVersion": "2",
  "irCanonicalVersion": "` + ir.CurrentVersion() + `",
  "artifacts": []
}`
	if err := os.WriteFile(filepath.Join(path, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	_, err := readArtifactHashManifest(root)
	if err == nil {
		t.Fatalf("expected compatibility error for stale fingerprint")
	}
	if !strings.Contains(err.Error(), "compiler fingerprint mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}
