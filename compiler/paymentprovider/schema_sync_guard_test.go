package paymentprovider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncSchema_preservesLocalExtensions(t *testing.T) {
	dir := t.TempDir()
	if _, err := SyncSchema(SchemaSyncOptions{ProjectPath: dir, CueRoot: ".cue"}); err != nil {
		t.Fatal(err)
	}
	profilesPath := filepath.Join(dir, ".cue", "schema", "profiles.cue")
	data, err := os.ReadFile(profilesPath)
	if err != nil {
		t.Fatal(err)
	}
	localExtra := "\nProfileLocalOnly: {\n\thas_payout: true\n}\n"
	if err := os.WriteFile(profilesPath, append(data, localExtra...), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := SyncSchema(SchemaSyncOptions{ProjectPath: dir, CueRoot: ".cue"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Guarded) == 0 {
		t.Fatalf("expected guarded profiles.cue, got written=%v skipped=%v", res.Written, res.Skipped)
	}
	after, err := os.ReadFile(profilesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(string(after), "ProfileLocalOnly") {
		t.Fatal("local-only profile was overwritten")
	}
}

func TestSyncSchema_forceOverwritesLocalExtensions(t *testing.T) {
	dir := t.TempDir()
	if _, err := SyncSchema(SchemaSyncOptions{ProjectPath: dir, CueRoot: ".cue"}); err != nil {
		t.Fatal(err)
	}
	profilesPath := filepath.Join(dir, ".cue", "schema", "profiles.cue")
	data, err := os.ReadFile(profilesPath)
	if err != nil {
		t.Fatal(err)
	}
	localExtra := "\nProfileLocalOnly: {\n\thas_payout: true\n}\n"
	if err := os.WriteFile(profilesPath, append(data, localExtra...), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := SyncSchema(SchemaSyncOptions{ProjectPath: dir, CueRoot: ".cue", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Guarded) != 0 {
		t.Fatalf("expected no guarded files with force, got %v", res.Guarded)
	}
	after, err := os.ReadFile(profilesPath)
	if err != nil {
		t.Fatal(err)
	}
	if containsString(string(after), "ProfileLocalOnly") {
		t.Fatal("local-only profile should have been overwritten with --force")
	}
}

func containsString(text, part string) bool {
	return len(part) == 0 || (len(text) >= len(part) && stringIndex(text, part) >= 0)
}

func stringIndex(text, part string) int {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
