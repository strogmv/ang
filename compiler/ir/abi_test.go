package ir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateABIV2_AcceptsGoldenFixture(t *testing.T) {
	t.Parallel()

	schema := mustReadABICompatFixture(t, "ir_v2_expected.json")
	if err := ValidateABIV2(schema); err != nil {
		t.Fatalf("ValidateABIV2 failed for v2 fixture: %v", err)
	}
}

func TestValidateABIV2_RejectsLegacyVersion(t *testing.T) {
	t.Parallel()

	schema := mustReadABICompatFixture(t, "ir_v1_expected.json")
	err := ValidateABIV2(schema)
	if err == nil {
		t.Fatalf("expected error for legacy version")
	}
	if !strings.Contains(err.Error(), "expected \"2\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateABIV2_RejectsNonJSONMetadata(t *testing.T) {
	t.Parallel()

	schema := mustReadABICompatFixture(t, "ir_v2_expected.json")
	schema.Entities[0].Metadata["bad"] = func() {}
	err := ValidateABIV2(schema)
	if err == nil {
		t.Fatalf("expected error for non-ABI metadata")
	}
	if !strings.Contains(err.Error(), "non-ABI value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateABIV2_CompatibilityFixtures(t *testing.T) {
	t.Parallel()

	for _, fixture := range []string{"ir_v0.json", "ir_v1_expected.json", "ir_v2_expected.json"} {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			schema := mustReadABICompatFixture(t, fixture)
			if err := MigrateToCurrent(schema); err != nil {
				t.Fatalf("MigrateToCurrent(%s): %v", fixture, err)
			}
			if err := ValidateABIV2(schema); err != nil {
				t.Fatalf("ValidateABIV2(%s): %v", fixture, err)
			}
		})
	}
}

func mustReadABICompatFixture(t *testing.T, name string) *Schema {
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
