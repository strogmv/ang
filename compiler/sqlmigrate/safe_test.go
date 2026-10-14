package sqlmigrate

import (
	"reflect"
	"strings"
	"testing"
)

// What Atlas plans when the migrations have hand-made objects that CUE does
// not declare, plus a few real additions.
const atlasPlan = `-- Drop index "uq_promocodes_company_code" from table: "promocodes"
DROP INDEX "uq_promocodes_company_code";
-- Modify "counterpartys" table
ALTER TABLE "counterpartys" DROP CONSTRAINT "counterpartys_bps_valid", ADD COLUMN "note" text NULL, ALTER COLUMN "bps" TYPE bigint, ADD COLUMN "rank" integer NOT NULL;
-- Create "widgets" table
CREATE TABLE "widgets" ("id" text NOT NULL, "label" text NOT NULL DEFAULT 'a;b', PRIMARY KEY ("id"));
-- Create index "idx_widgets_label" to table: "widgets"
CREATE INDEX "idx_widgets_label" ON "widgets" ("label");
-- Create index "idx_widgets_big" to table: "widgets"
CREATE UNIQUE INDEX "idx_widgets_big" ON "widgets" ("id", "label") WHERE (label <> ''::text);
-- Modify "widgets" table
ALTER TABLE "widgets" ADD CONSTRAINT "widgets_label_valid" CHECK (label <> 'x');
-- Drop "legacy" table
DROP TABLE "legacy";
CREATE FUNCTION f() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END; $$;
CREATE EXTENSION "pg_trgm";
ALTER TABLE "widgets" RENAME COLUMN "label" TO "title";
`

func TestSafeKeepsOnlyAdditiveIdempotentStatements(t *testing.T) {
	p := Safe(atlasPlan, map[string]bool{"idx_widgets_big": true})
	want := []string{
		`ALTER TABLE "counterpartys" ADD COLUMN IF NOT EXISTS "note" text NULL, ADD COLUMN IF NOT EXISTS "rank" integer NOT NULL;`,
		`CREATE TABLE IF NOT EXISTS "widgets" ("id" text NOT NULL, "label" text NOT NULL DEFAULT 'a;b', PRIMARY KEY ("id"));`,
		`CREATE INDEX IF NOT EXISTS "idx_widgets_label" ON "widgets" ("label");`,
		`CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS "idx_widgets_big" ON "widgets" ("id", "label") WHERE (label <> ''::text);`,
		`DO $ang$ BEGIN ALTER TABLE "widgets" ADD CONSTRAINT "widgets_label_valid" CHECK (label <> 'x'); EXCEPTION WHEN duplicate_object OR duplicate_table THEN NULL; END $ang$;`,
		`CREATE OR REPLACE FUNCTION f() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END; $$;`,
		`CREATE EXTENSION IF NOT EXISTS "pg_trgm";`,
	}
	if !reflect.DeepEqual(p.Statements, want) {
		t.Fatalf("statements:\n%s\nwant:\n%s", strings.Join(p.Statements, "\n"), strings.Join(want, "\n"))
	}
	if !p.NoTransaction {
		t.Fatal("a concurrent index needs a file without a transaction")
	}
	warnings := strings.Join(p.Warnings, "\n")
	for _, w := range []string{
		`DROP INDEX "uq_promocodes_company_code"`,
		`DROP CONSTRAINT "counterpartys_bps_valid"`,
		`ALTER COLUMN "bps" TYPE bigint`,
		`DROP TABLE "legacy"`,
		`RENAME COLUMN "label" TO "title"`,
		`"rank" integer NOT NULL: NOT NULL without DEFAULT`,
	} {
		if !strings.Contains(warnings, w) {
			t.Errorf("no warning for %s in:\n%s", w, warnings)
		}
	}
	rendered := Render(p)
	for _, line := range strings.Split(rendered, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "DROP ") || strings.Contains(upper, "RENAME") || strings.Contains(upper, "ALTER COLUMN") {
			t.Fatalf("destructive statement in the migration: %s", line)
		}
	}
	if !strings.HasPrefix(rendered, "-- atlas:txmode none\n") {
		t.Fatalf("missing txmode directive:\n%s", rendered)
	}
}

func TestSafeOfNothing(t *testing.T) {
	p := Safe("", nil)
	if synced := Safe("Schemas are synced, no changes to be made.\n", nil); len(synced.Statements)+len(synced.Warnings) != 0 {
		t.Fatalf("%#v", synced)
	}
	if len(p.Statements) != 0 || len(p.Warnings) != 0 || p.NoTransaction {
		t.Fatalf("%#v", p)
	}
}

func TestSplitStatementsKeepsQuotedSemicolons(t *testing.T) {
	got := SplitStatements("-- c;\nSELECT 'a;b'; SELECT \"x;y\" FROM t; /* ; */ DO $f$ BEGIN PERFORM 1; END $f$;")
	want := []string{"SELECT 'a;b';", `SELECT "x;y" FROM t;`, "DO $f$ BEGIN PERFORM 1; END $f$;"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%q", got)
	}
}

func TestConcurrentIndexNames(t *testing.T) {
	schema := "CREATE INDEX IF NOT EXISTS idx_a ON a (x); -- ang:concurrently\nCREATE INDEX IF NOT EXISTS idx_b ON b (x);\n"
	if got := ConcurrentIndexNames(schema); !reflect.DeepEqual(got, map[string]bool{"idx_a": true}) {
		t.Fatalf("%v", got)
	}
}

func TestDevURL(t *testing.T) {
	if DevURL("") != "docker://postgres/18/dev" || DevURL("17") != "docker://postgres/17/dev" {
		t.Fatal(DevURL(""))
	}
	if !ValidPostgresVersion("") || !ValidPostgresVersion("18") || ValidPostgresVersion("18/dev\"") {
		t.Fatal("ValidPostgresVersion")
	}
}

func TestStripRawDDL(t *testing.T) {
	schema := "CREATE TABLE IF NOT EXISTS a (id text);\n-- ang:ddl AuditLog\nCREATE OR REPLACE FUNCTION f() RETURNS trigger AS $$ BEGIN RETURN NEW; END $$ LANGUAGE plpgsql;\n-- ang:ddl end\nCREATE INDEX IF NOT EXISTS i ON a (id);\n"
	got, entities := StripRawDDL(schema)
	if got != "CREATE TABLE IF NOT EXISTS a (id text);\nCREATE INDEX IF NOT EXISTS i ON a (id);\n" {
		t.Fatalf("%q", got)
	}
	if !reflect.DeepEqual(entities, []string{"AuditLog"}) {
		t.Fatalf("%v", entities)
	}
}
