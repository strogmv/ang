package ir

import "testing"

func TestMigrateToCurrent_UpgradesLegacySchema(t *testing.T) {
	schema := &Schema{}
	if err := MigrateToCurrent(schema); err != nil {
		t.Fatalf("migrate legacy schema: %v", err)
	}
	if schema.IRVersion != IRVersionV1 {
		t.Fatalf("expected ir version %s, got %s", IRVersionV1, schema.IRVersion)
	}
	if schema.Metadata == nil {
		t.Fatalf("expected metadata map to be initialized")
	}
}

func TestMigrateToCurrent_RejectsUnknownVersion(t *testing.T) {
	schema := &Schema{IRVersion: "99"}
	if err := MigrateToCurrent(schema); err == nil {
		t.Fatalf("expected error for unknown version")
	}
}
