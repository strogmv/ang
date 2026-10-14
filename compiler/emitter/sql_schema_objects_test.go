package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

func sqlTestField(name string, index bool) ir.Field {
	f := ir.Field{Name: name, Type: ir.TypeRef{Kind: ir.KindString}}
	if index {
		f.Attributes = []ir.Attribute{{Name: "db", Args: map[string]any{"index": true}}}
	}
	return f
}

func sqlTestEntity(indexes ...ir.Index) ir.Entity {
	return ir.Entity{
		Name:    "Customer",
		Source:  "cue/domain/customer.cue:3",
		Fields:  []ir.Field{sqlTestField("id", false), sqlTestField("companyId", false), sqlTestField("email", false), sqlTestField("tags", false)},
		Indexes: indexes,
	}
}

func emitSchemaSQL(t *testing.T, em *Emitter, entities ...ir.Entity) string {
	t.Helper()
	if err := em.EmitSQL(entities); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(em.OutputDir, "db", "schema", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func emitSchemaSQLErr(t *testing.T, em *Emitter, entities ...ir.Entity) error {
	t.Helper()
	return em.EmitSQL(entities)
}

func TestEmitSQLIndexFeatures(t *testing.T) {
	cases := []struct {
		name  string
		index ir.Index
		want  string
	}{
		{"partial", ir.Index{Name: "uq_customers_open", Fields: []string{"companyId"}, Unique: true, Where: "email <> ''"},
			"CREATE UNIQUE INDEX IF NOT EXISTS uq_customers_open ON customers (companyid) WHERE email <> '';"},
		{"gin", ir.Index{Fields: []string{"tags"}, Using: "GIN"},
			"CREATE INDEX IF NOT EXISTS idx_customers_tags ON customers USING gin (tags);"},
		{"expression", ir.Index{Name: "users_email_lower_key", Expr: "lower(email)", Unique: true},
			"CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_key ON customers (lower(email));"},
		{"mixed expression", ir.Index{Name: "uq_customers_ext", Expr: "companyid, lower(btrim(email))", Unique: true, Where: "btrim(email) <> ''"},
			"CREATE UNIQUE INDEX IF NOT EXISTS uq_customers_ext ON customers (companyid, lower(btrim(email))) WHERE btrim(email) <> '';"},
		{"opclass", ir.Index{Fields: []string{"companyId", "email"}, Opclass: "text_pattern_ops"},
			"CREATE INDEX IF NOT EXISTS idx_customers_companyid_email ON customers (companyid text_pattern_ops, email text_pattern_ops);"},
		{"named plain", ir.Index{Name: "ix_customers_company", Fields: []string{"companyId"}},
			"CREATE INDEX IF NOT EXISTS ix_customers_company ON customers (companyid);"},
		{"concurrently", ir.Index{Fields: []string{"email"}, Using: "btree", Concurrently: true},
			"CREATE INDEX IF NOT EXISTS idx_customers_email ON customers USING btree (email); -- ang:concurrently"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := emitSchemaSQL(t, New(t.TempDir(), "", "templates"), sqlTestEntity(tc.index))
			if !strings.Contains(schema, "\n"+tc.want+"\n") {
				t.Fatalf("missing\n%s\nin:\n%s", tc.want, schema)
			}
		})
	}
}

func TestEmitSQLIndexRejectsBadDeclarations(t *testing.T) {
	cases := []struct {
		name  string
		index ir.Index
		want  string
	}{
		{"expr without name", ir.Index{Expr: "lower(email)"}, "needs name"},
		{"fields and expr", ir.Index{Name: "x", Fields: []string{"email"}, Expr: "lower(email)"}, "either fields or expr"},
		{"unknown method", ir.Index{Fields: []string{"email"}, Using: "fulltext"}, "unknown index method"},
		{"long explicit name", ir.Index{Name: strings.Repeat("a", 64), Fields: []string{"email"}}, "Postgres allows 63"},
		{"quoted name", ir.Index{Name: `Bad"Name`, Fields: []string{"email"}}, "lower-case SQL identifier"},
		{"statement in where", ir.Index{Fields: []string{"email"}, Where: "true; DROP TABLE customers"}, "must not contain ';'"},
		{"opclass with expr", ir.Index{Name: "x", Expr: "lower(email)", Opclass: "text_pattern_ops"}, "write it inside expr"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := emitSchemaSQLErr(t, New(t.TempDir(), "", "templates"), sqlTestEntity(tc.index))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestEmitSQLChecksAndDDL(t *testing.T) {
	ent := sqlTestEntity()
	ent.Checks = []ir.Check{{Name: "customers_email_valid", Expr: "email <> ''"}}
	ent.DDL = []string{"CREATE OR REPLACE FUNCTION customers_touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;"}
	schema := emitSchemaSQL(t, New(t.TempDir(), "", "templates"), ent)
	for _, want := range []string{
		"\nDO $ang$ BEGIN ALTER TABLE customers ADD CONSTRAINT customers_email_valid CHECK (email <> ''); EXCEPTION WHEN duplicate_object THEN NULL; END $ang$;\n",
		"\n-- ang:ddl Customer\nCREATE OR REPLACE FUNCTION customers_touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;\n-- ang:ddl end\n",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("missing %q in:\n%s", want, schema)
		}
	}
	ent.Checks = append(ent.Checks, ent.Checks[0])
	if err := emitSchemaSQLErr(t, New(t.TempDir(), "", "templates"), ent); err == nil || !strings.Contains(err.Error(), "two checks are named") {
		t.Fatalf("duplicate check name: err = %v", err)
	}
}

// Postgres keeps 63 bytes of a name. By default a longer generated name is
// emitted as before (existing databases hold its truncated form) with a
// warning that names the stored name; "hashed" shortens it deterministically.
func TestEmitSQLLongIndexNames(t *testing.T) {
	ent := ir.Entity{Name: "ProductVariantPrice", Fields: []ir.Field{
		sqlTestField("id", false), sqlTestField("companyId", false), sqlTestField("variantId", false),
		sqlTestField("priceListId", false), sqlTestField("marketId", false), sqlTestField("customerCompanyId", false),
	}, Indexes: []ir.Index{{Fields: []string{"companyId", "variantId", "priceListId", "marketId", "customerCompanyId"}, Unique: true}}}
	long := "uidx_productvariantprices_companyid_variantid_pricelistid_marketid_customercompanyid"

	em := New(t.TempDir(), "", "templates")
	var warnings []normalizer.Warning
	em.WarningSink = func(w normalizer.Warning) { warnings = append(warnings, w) }
	schema := emitSchemaSQL(t, em, ent)
	if !strings.Contains(schema, "IF NOT EXISTS "+long+" ON") {
		t.Fatalf("default naming changed:\n%s", schema)
	}
	if len(warnings) != 1 || warnings[0].Code != CodeSQLIndexNameTruncated || !strings.Contains(warnings[0].Message, long[:63]) {
		t.Fatalf("warnings = %#v", warnings)
	}

	em = New(t.TempDir(), "", "templates")
	em.SQLIndexNames = SQLIndexNamesHashed
	schema = emitSchemaSQL(t, em, ent)
	hashed := hashedSQLName(long)
	if len(hashed) != 63 || !strings.HasPrefix(hashed, long[:54]+"_") || hashed != hashedSQLName(long) {
		t.Fatalf("hashed name %q", hashed)
	}
	if !strings.Contains(schema, "IF NOT EXISTS "+hashed+" ON") {
		t.Fatalf("hashed name %s missing:\n%s", hashed, schema)
	}

	// Pinning the stored name keeps the database as it is, without a warning.
	ent.Indexes[0].Name = long[:63]
	em = New(t.TempDir(), "", "templates")
	warnings = nil
	em.WarningSink = func(w normalizer.Warning) { warnings = append(warnings, w) }
	schema = emitSchemaSQL(t, em, ent)
	if !strings.Contains(schema, "IF NOT EXISTS "+long[:63]+" ON") || len(warnings) != 0 {
		t.Fatalf("pinned name: warnings %#v\n%s", warnings, schema)
	}
}

// Two different indexes whose names Postgres stores alike: IF NOT EXISTS
// would skip the second without a word. The build fails instead.
func TestEmitSQLIndexNameCollision(t *testing.T) {
	// idx_customers_<50 b>one_id and idx_customers_<50 b>two_id differ only
	// after byte 63.
	one, two := strings.Repeat("b", 50)+"one", strings.Repeat("b", 50)+"two"
	ent := ir.Entity{Name: "Customer", Fields: []ir.Field{
		sqlTestField("id", false), sqlTestField(one, false), sqlTestField(two, false),
	}, Indexes: []ir.Index{
		{Fields: []string{one, "id"}},
		{Fields: []string{two, "id"}},
	}}
	err := emitSchemaSQLErr(t, New(t.TempDir(), "", "templates"), ent)
	if err == nil || !strings.Contains(err.Error(), "SQL_INDEX_NAME_COLLISION") {
		t.Fatalf("err = %v", err)
	}

	// The hashed mode keeps them apart.
	em := New(t.TempDir(), "", "templates")
	em.SQLIndexNames = SQLIndexNamesHashed
	if err := em.EmitSQL([]ir.Entity{ent}); err != nil {
		t.Fatalf("hashed: %v", err)
	}

	// An explicit name that a field index already uses for something else.
	ent2 := sqlTestEntity(ir.Index{Name: "idx_customers_email", Fields: []string{"companyId"}})
	ent2.Fields[2] = sqlTestField("email", true)
	err = emitSchemaSQLErr(t, New(t.TempDir(), "", "templates"), ent2)
	if err == nil || !strings.Contains(err.Error(), "SQL_INDEX_NAME_COLLISION") {
		t.Fatalf("explicit name collision: err = %v", err)
	}

	// The same index declared twice (a field index and a one-field composite)
	// stays a harmless duplicate, as before.
	ent3 := sqlTestEntity(ir.Index{Fields: []string{"email"}})
	ent3.Fields[2] = sqlTestField("email", true)
	if err := New(t.TempDir(), "", "templates").EmitSQL([]ir.Entity{ent3}); err != nil {
		t.Fatalf("identical duplicate: %v", err)
	}

	// An index named like a table.
	ent4 := sqlTestEntity(ir.Index{Name: "customers", Fields: []string{"email"}})
	if err := emitSchemaSQLErr(t, New(t.TempDir(), "", "templates"), ent4); err == nil || !strings.Contains(err.Error(), "name of a table") {
		t.Fatalf("table name: err = %v", err)
	}
}

func TestEmitInfraConfigsPostgresVersion(t *testing.T) {
	for version, want := range map[string]string{"": "docker://postgres/18/dev", "17": "docker://postgres/17/dev"} {
		root := t.TempDir()
		em := New(root, "", "templates")
		em.PostgresVersion = version
		if err := em.EmitInfraConfigs(); err != nil {
			t.Fatal(err)
		}
		hcl, _ := os.ReadFile(filepath.Join(root, "atlas.hcl"))
		if !strings.Contains(string(hcl), `dev = "`+want+`"`) {
			t.Fatalf("version %q: atlas.hcl\n%s", version, hcl)
		}
		script, _ := os.ReadFile(filepath.Join(root, "scripts", "atlas-diff.sh"))
		if strings.Contains(string(script), "atlas migrate diff") || !strings.Contains(string(script), "migrate diff") {
			t.Fatalf("atlas-diff.sh must go through ang migrate diff:\n%s", script)
		}
	}
}
