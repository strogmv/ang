package paymentprovider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledSchemaFiles(t *testing.T) {
	files, err := BundledSchemaFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 {
		t.Fatalf("expected provider.cue and catalogs.cue, got %v", files)
	}
	seen := map[string]bool{}
	for _, f := range files {
		seen[f] = true
	}
	for _, want := range []string{"provider.cue", "catalogs.cue", "profiles.cue"} {
		if !seen[want] {
			t.Fatalf("missing bundled schema file %q", want)
		}
	}
}

func TestBundledSchemaSupportsGatewayPayoutFields(t *testing.T) {
	providerSchema, err := ReadBundledSchemaFile("provider.cue")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`algorithm:       "sha256" | "hmac-sha1"`,
		`"username_key_body_b64"`,
		`"hmac_timestamp_nonce"`,
		`"sha256_concat"`,
		`username_header: *"" | string`,
		`username_key:    *"" | string`,
		`payout_status_request: *null | #RequestDef`,
	} {
		if !containsString(string(providerSchema), want) {
			t.Fatalf("provider schema missing %q", want)
		}
	}

	catalogs, err := ReadBundledSchemaFile("catalogs.cue")
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(string(catalogs), `"tx_payment_method" | "foreign_id"`) {
		t.Fatal("catalog schema missing foreign_id request source")
	}
}

func TestSyncSchema_andCheckSchema(t *testing.T) {
	dir := t.TempDir()
	res, err := SyncSchema(SchemaSyncOptions{ProjectPath: dir, CueRoot: ".cue"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) == 0 {
		t.Fatalf("expected writes on first sync, got skipped=%v", res.Skipped)
	}
	for _, name := range res.Written {
		if _, err := os.Stat(filepath.Join(dir, ".cue", "schema", name)); err != nil {
			t.Fatalf("missing synced file %q: %v", name, err)
		}
	}

	drift, err := CheckSchema(dir, ".cue")
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Fatalf("unexpected drift after sync: %v", drift)
	}

	res2, err := SyncSchema(SchemaSyncOptions{ProjectPath: dir, CueRoot: ".cue"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Written) != 0 {
		t.Fatalf("expected no writes on second sync, wrote %v", res2.Written)
	}
	if len(res2.Skipped) == 0 {
		t.Fatalf("expected skipped files on second sync")
	}
}

func TestCheckSchema_detectsDrift(t *testing.T) {
	dir := t.TempDir()
	if _, err := SyncSchema(SchemaSyncOptions{ProjectPath: dir, CueRoot: ".cue"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".cue", "schema", "catalogs.cue")
	if err := os.WriteFile(path, []byte("// drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := CheckSchema(dir, ".cue")
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) == 0 {
		t.Fatal("expected drift")
	}
}
