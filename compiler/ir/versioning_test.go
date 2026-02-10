package ir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMigrateToCurrent_RejectsUnknownVersion(t *testing.T) {
	schema := &Schema{IRVersion: "99"}
	if err := MigrateToCurrent(schema); err == nil {
		t.Fatalf("expected error for unknown version")
	}
}

func TestMigrateV0ToV1_Fixture(t *testing.T) {
	input := mustReadSchemaFixture(t, "ir_v0.json")
	if err := MigrateToCurrent(input); err != nil {
		t.Fatalf("migrate legacy schema: %v", err)
	}

	expected := mustReadSchemaFixture(t, "ir_v1_expected.json")
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
	expectedBytes, err := os.ReadFile(filepath.Join("testdata", "ir_v1_expected.json"))
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
