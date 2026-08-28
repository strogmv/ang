package paymentprovider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_sharedSchemaDir(t *testing.T) {
	root := filepath.Join("testdata", "pumpp2p")
	sharedSchema := filepath.Join("testdata", "pumpp2p", ".cue", "schema")
	spec, err := Load(root, ".cue", sharedSchema)
	if err != nil {
		t.Fatalf("Load with shared schema: %v", err)
	}
	if spec.SID != "pumpp2p" {
		t.Fatalf("sid: %q", spec.SID)
	}
}

func TestLoadProjectConfig_schemaDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ang.yaml"), []byte(`
cue_root: ".cue"
templates_dir: "../.ang/templates"
schema_dir: "../.ang/schema"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pc := LoadProjectConfig(dir)
	if pc.SchemaDir != "../.ang/schema" {
		t.Fatalf("schema_dir: %q", pc.SchemaDir)
	}
	resolved, err := ResolvePath(dir, pc.SchemaDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(resolved)) != ".ang" {
		t.Fatalf("resolved: %s", resolved)
	}
}

func TestLoad_rejectsLocalSchemaWhenSchemaDirSet(t *testing.T) {
	dir := t.TempDir()
	cueDir := filepath.Join(dir, ".cue")
	if err := os.MkdirAll(filepath.Join(cueDir, "schema"), 0o755); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(dir, "shared-schema")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	err := rejectStaleLocalSchema(cueDir, shared)
	if err == nil {
		t.Fatal("expected error for leftover .cue/schema")
	}
}
