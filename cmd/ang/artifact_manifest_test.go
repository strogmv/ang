package main

import (
	"os"
	"path/filepath"
	"testing"
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
	m1, err := buildArtifactHashManifest(root, targets, "2", "input-hash", "template-hash")
	if err != nil {
		t.Fatalf("first manifest: %v", err)
	}
	m2, err := buildArtifactHashManifest(root, targets, "2", "input-hash", "template-hash")
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
