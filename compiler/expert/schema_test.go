package expert

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

func TestKnowledgeSchemaBuilds(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	schemaDir := filepath.Join(filepath.Dir(file), "..", "..", "cue", "schema")
	instances := load.Instances([]string{"."}, &load.Config{Dir: schemaDir})
	if len(instances) != 1 {
		t.Fatalf("loaded %d CUE instances, want 1", len(instances))
	}
	if instances[0].Err != nil {
		t.Fatalf("load schema: %v", instances[0].Err)
	}
	value := cuecontext.New().BuildInstance(instances[0])
	if err := value.Err(); err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if !value.LookupPath(cue.ParsePath("#ExpertKnowledgePack")).Exists() {
		t.Fatal("#ExpertKnowledgePack is missing")
	}
}

func TestLoadKnowledgePack(t *testing.T) {
	root := t.TempDir()
	writeKnowledgeTestFile(t, filepath.Join(root, "cue.mod", "module.cue"), `module: "example.com/expert"
language: {
	version: "v0.9.0"
}
`)
	schemaSource, err := os.ReadFile(filepath.Join(repoRoot(t), "cue", "schema", "expert.cue"))
	if err != nil {
		t.Fatalf("read expert schema: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(root, "schema", "expert.cue"), string(schemaSource))
	packDir := filepath.Join(root, "packs", "security")
	writeKnowledgeTestFile(t, filepath.Join(packDir, "security.cue"), `package security

import "example.com/expert/schema"

pack: schema.#ExpertKnowledgePack & {
	schema: "ang/knowledge-pack/v1"
	name: "security"
	version: "v1"
	rules: [{
		id: "security.auth.endpoint_requires_actor"
		version: "v1"
		conditions: [{op: "fact_exists", fact_kind: "endpoint"}]
		conclusions: [{kind: "finding", code: "AUTH_ACTOR_REQUIRED", severity: "warning", summary: "endpoint needs actor"}]
	}]
}
`)

	pack, err := LoadKnowledgePack(packDir)
	if err != nil {
		t.Fatalf("LoadKnowledgePack: %v", err)
	}
	if pack.Name != "security" || len(pack.Rules) != 1 || pack.Rules[0].BaseConfidence != 0.5 {
		t.Fatalf("unexpected decoded pack: %#v", pack)
	}

	invalidDir := filepath.Join(root, "packs", "invalid")
	writeKnowledgeTestFile(t, filepath.Join(invalidDir, "invalid.cue"), `package invalid

import "example.com/expert/schema"

pack: schema.#ExpertKnowledgePack & {
	schema: "ang/knowledge-pack/v1"
	name: "invalid"
	version: "v1"
	rules: [{
		id: "security.invalid"
		version: "v1"
		conditions: [{op: "fact_exists", fact_kind: "endpoint", state: "known"}]
		conclusions: [{kind: "finding", code: "INVALID", severity: "warning", summary: "invalid"}]
	}]
}
`)
	if _, err := LoadKnowledgePack(invalidDir); err == nil {
		t.Fatal("LoadKnowledgePack accepted a field irrelevant to fact_exists")
	}
}

func writeKnowledgeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}
