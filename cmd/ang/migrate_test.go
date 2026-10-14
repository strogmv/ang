package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ang migrate diff writes only additions: an index that exists only in the
// migrations (hand-made, not in CUE) is a warning, never a DROP.
func TestMigrateDiffWritesOnlyAdditions(t *testing.T) {
	dir := t.TempDir()
	migrations := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrations, 0o755); err != nil {
		t.Fatal(err)
	}
	schema := filepath.Join(dir, "schema.sql")
	if err := os.WriteFile(schema, []byte("CREATE INDEX IF NOT EXISTS idx_t_b ON t (b); -- ang:concurrently\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotDev string
	old := atlasSchemaDiff
	defer func() { atlasSchemaDiff = old }()
	atlasSchemaDiff = func(from, to, devURL string) (string, error) {
		gotDev = devURL
		return "DROP INDEX \"uq_hand_made\";\nALTER TABLE \"t\" ADD COLUMN \"b\" text NULL;\nCREATE INDEX \"idx_t_b\" ON \"t\" (\"b\");\n", nil
	}
	var stdout, stderr bytes.Buffer
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	err := runSafeMigrateDiff(migrateDiffOptions{Name: "add_b", DevURL: "postgres://dev", Dir: migrations, To: schema, Now: now}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if gotDev != "postgres://dev" {
		t.Fatalf("dev url %q", gotDev)
	}
	data, err := os.ReadFile(filepath.Join(migrations, "20261014120000_add_b.sql"))
	if err != nil {
		t.Fatal(err)
	}
	file := string(data)
	for _, want := range []string{
		`ALTER TABLE "t" ADD COLUMN IF NOT EXISTS "b" text NULL;`,
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS "idx_t_b" ON "t" ("b");`,
		"-- atlas:txmode none",
	} {
		if !strings.Contains(file, want) {
			t.Fatalf("missing %q in:\n%s", want, file)
		}
	}
	for _, line := range strings.Split(file, "\n") {
		if strings.HasPrefix(line, "DROP") {
			t.Fatalf("destructive statement written: %s", line)
		}
	}
	if !strings.Contains(stderr.String(), `WARNING: skipped (not additive): DROP INDEX "uq_hand_made";`) {
		t.Fatalf("stderr: %s", stderr.String())
	}

	// A baseline dated in the future: the new migration still sorts last.
	if err := os.WriteFile(filepath.Join(migrations, "20261101000000_baseline.sql"), []byte("SELECT 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSafeMigrateDiff(migrateDiffOptions{Name: "later", DevURL: "x", Dir: migrations, To: schema, Now: now}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(migrations, "20261101000001_later.sql")); err != nil {
		t.Fatalf("version after the newest migration: %v", err)
	}

	// Only drops planned: no file at all.
	atlasSchemaDiff = func(string, string, string) (string, error) { return "DROP INDEX \"uq_hand_made\";\n", nil }
	stdout.Reset()
	if err := runSafeMigrateDiff(migrateDiffOptions{Name: "nothing", DevURL: "x", Dir: migrations, To: schema, Now: now}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(migrations, "20261014120000_nothing.sql")); !os.IsNotExist(err) {
		t.Fatalf("a migration was written for drops only: %v", err)
	}
	if !strings.Contains(stdout.String(), "No additive changes") {
		t.Fatalf("stdout: %s", stdout.String())
	}
}

func TestAtlasDevURLPrefersFlagThenEnvThenAtlasHCL(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANG_ATLAS_DEV_URL", "")
	if got := atlasDevURL(""); got != "docker://postgres/18/dev" {
		t.Fatalf("default %q", got)
	}
	_ = os.WriteFile("atlas.hcl", []byte("env \"local\" {\n  dev = \"docker://postgres/17/dev\"\n}\n"), 0o644)
	if got := atlasDevURL(""); got != "docker://postgres/17/dev" {
		t.Fatalf("atlas.hcl %q", got)
	}
	t.Setenv("ANG_ATLAS_DEV_URL", "postgres://env")
	if got := atlasDevURL(""); got != "postgres://env" {
		t.Fatalf("env %q", got)
	}
	if got := atlasDevURL("postgres://flag"); got != "postgres://flag" {
		t.Fatalf("flag %q", got)
	}
}
