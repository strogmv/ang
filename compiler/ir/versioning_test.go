package ir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMigrateToCurrent_RejectsUnknownVersion(t *testing.T) {
	schema := &Schema{IRVersion: "99"}
	if err := MigrateToCurrent(schema); err == nil {
		t.Fatalf("expected error for unknown version")
	}
}

func TestMigrateV0ToCurrent_Fixture(t *testing.T) {
	input := mustReadSchemaFixture(t, "ir_v0.json")
	if err := MigrateToCurrent(input); err != nil {
		t.Fatalf("migrate legacy schema: %v", err)
	}

	expected := mustReadSchemaFixture(t, "ir_v2_expected.json")
	if !reflect.DeepEqual(expected, input) {
		gotJSON, _ := json.MarshalIndent(input, "", "  ")
		wantJSON, _ := json.MarshalIndent(expected, "", "  ")
		t.Fatalf("migration mismatch\nwant:\n%s\ngot:\n%s", string(wantJSON), string(gotJSON))
	}
}

func TestToCanonicalJSON_NormalizesVersion(t *testing.T) {
	input := mustReadSchemaFixture(t, "ir_v0.json")
	got, err := ToCanonicalJSON(input)
	if err != nil {
		t.Fatalf("canonical json: %v", err)
	}
	expectedBytes, err := os.ReadFile(filepath.Join("testdata", "ir_v2_expected.json"))
	if err != nil {
		t.Fatalf("read expected fixture: %v", err)
	}
	var expectedAny any
	if err := json.Unmarshal(expectedBytes, &expectedAny); err != nil {
		t.Fatalf("decode expected fixture: %v", err)
	}
	var gotAny any
	if err := json.Unmarshal(got, &gotAny); err != nil {
		t.Fatalf("decode canonical output: %v", err)
	}
	if !reflect.DeepEqual(expectedAny, gotAny) {
		t.Fatalf("canonical output differs from expected fixture")
	}
}

func TestMigrateV1ToCurrent_Fixture(t *testing.T) {
	input := mustReadSchemaFixture(t, "ir_v1_expected.json")
	if err := MigrateToCurrent(input); err != nil {
		t.Fatalf("migrate v1 schema: %v", err)
	}
	if got := input.IRVersion; got != IRVersionV2 {
		t.Fatalf("expected ir_version=%s, got %s", IRVersionV2, got)
	}
}

func TestMigrateToCurrent_Idempotent(t *testing.T) {
	input := mustReadSchemaFixture(t, "ir_v2_expected.json")
	if err := MigrateToCurrent(input); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	first, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	if err := MigrateToCurrent(input); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	second, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected migration to be idempotent")
	}
}

func TestRegisteredMigrations_ContainsV1ToV2(t *testing.T) {
	t.Parallel()

	edges := RegisteredMigrations()
	joined := strings.Join(edges, ",")
	if !strings.Contains(joined, "1->2") {
		t.Fatalf("expected migration edge 1->2, got %v", edges)
	}
}

func TestMigrateToVersion_V1ToV2(t *testing.T) {
	t.Parallel()

	input := mustReadSchemaFixture(t, "ir_v1_expected.json")
	if err := MigrateToVersion(input, IRVersionV2); err != nil {
		t.Fatalf("MigrateToVersion failed: %v", err)
	}
	if input.IRVersion != IRVersionV2 {
		t.Fatalf("ir_version=%s, want %s", input.IRVersion, IRVersionV2)
	}
}

func mustReadSchemaFixture(t *testing.T, name string) *Schema {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return &schema
}
