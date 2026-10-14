package normalizer

import (
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func parseEntityCUE(t *testing.T, src string) (Entity, error) {
	t.Helper()
	val := cuecontext.New().CompileString(src)
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	return New().parseEntity("Customer", val.LookupPath(cuePath("Customer")))
}

func TestParseEntityIndexKeys(t *testing.T) {
	ent, err := parseEntityCUE(t, `
Customer: {
	id:        string
	companyId: string
	email:     string
	tags:      [...string]
	indexes: [
		{fields: ["companyId"], unique: true},
		{name: "uq_customers_email_lower", expr: "lower(email)", unique: true, where: "email <> ''", concurrently: true},
		{fields: ["tags"], using: "gin"},
		{fields: ["email"], opclass: "text_pattern_ops"},
	]
}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ent.Indexes) != 4 {
		t.Fatalf("indexes = %#v", ent.Indexes)
	}
	got := ent.Indexes[1]
	got.Source = ""
	want := IndexDef{Name: "uq_customers_email_lower", Expr: "lower(email)", Unique: true, Where: "email <> ''", Concurrently: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expression index = %#v, want %#v", got, want)
	}
	if ent.Indexes[2].Using != "gin" || ent.Indexes[3].Opclass != "text_pattern_ops" {
		t.Fatalf("using/opclass lost: %#v", ent.Indexes)
	}
	for _, f := range ent.Fields {
		if f.Name == "indexes" {
			t.Fatal("indexes became a field")
		}
	}
}

// A typo in an index key would silently build a different index (a partial
// index without its WHERE); it is an error with the position instead.
func TestParseEntityIndexRejectsUnknownKey(t *testing.T) {
	_, err := parseEntityCUE(t, `
Customer: {
	id: string
	indexes: [{fields: ["id"], wher: "id <> ''"}]
}`)
	if err == nil || !strings.Contains(err.Error(), `unknown index key "wher"`) {
		t.Fatalf("err = %v", err)
	}
	_, err = parseEntityCUE(t, `
Customer: {
	id: string
	indexes: [{fields: ["id"], unique: "yes"}]
}`)
	if err == nil || !strings.Contains(err.Error(), "unique must be a bool") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseEntityChecksAndDDL(t *testing.T) {
	ent, err := parseEntityCUE(t, `
Customer: {
	@check(name="customers_bps_valid", expr="(bps >= 0) AND (bps <= 1000000)")
	@check(name="customers_mode_valid", expr="mode = ANY (ARRAY['markup'::text, 'discount'::text])")
	id:   string
	bps:  int
	mode: string
	ddl: """
		CREATE OR REPLACE FUNCTION customers_touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;
		"""
}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ent.Checks) != 2 || ent.Checks[0].Name != "customers_bps_valid" || ent.Checks[0].Expr != "(bps >= 0) AND (bps <= 1000000)" ||
		ent.Checks[1].Expr != "mode = ANY (ARRAY['markup'::text, 'discount'::text])" {
		t.Fatalf("checks = %#v", ent.Checks)
	}
	if len(ent.DDL) != 1 || !strings.HasPrefix(ent.DDL[0], "CREATE OR REPLACE FUNCTION customers_touch()") {
		t.Fatalf("ddl = %#v", ent.DDL)
	}
	for _, f := range ent.Fields {
		if f.Name == "ddl" {
			t.Fatal("ddl with SQL became a field")
		}
	}
}

// A field that happens to be called ddl (a type, not SQL) stays a field.
func TestParseEntityFieldNamedDDLStaysAField(t *testing.T) {
	ent, err := parseEntityCUE(t, `
Customer: {
	id:  string
	ddl: string
}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ent.DDL) != 0 {
		t.Fatalf("ddl = %#v", ent.DDL)
	}
	found := false
	for _, f := range ent.Fields {
		found = found || f.Name == "ddl"
	}
	if !found {
		t.Fatal("field ddl lost")
	}
}

func TestParseEntityCheckNeedsNameAndExpr(t *testing.T) {
	_, err := parseEntityCUE(t, `
Customer: {
	@check(name="x")
	id: string
}`)
	if err == nil || !strings.Contains(err.Error(), "@check needs") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractTargetReadsSQLOptions(t *testing.T) {
	val := cuecontext.New().CompileString(`state: target: {lang: "go", sql_index_names: "hashed", postgres_version: 18}`)
	td, err := New().ExtractTarget(val)
	if err != nil {
		t.Fatal(err)
	}
	if td.SQLIndexNames != "hashed" || td.PostgresVersion != "18" {
		t.Fatalf("target = %#v", td)
	}
}

func cuePath(s string) cue.Path { return cue.ParsePath(s) }
