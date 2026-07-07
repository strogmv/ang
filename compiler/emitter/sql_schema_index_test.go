package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitSQLIncludesCompositeUniqueIndex(t *testing.T) {
	root := t.TempDir()
	em := New(root, "", "templates")
	entities := []ir.Entity{{
		Name: "Membership",
		Fields: []ir.Field{
			{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
			{Name: "company_id", Type: ir.TypeRef{Kind: ir.KindString}},
			{Name: "user_id", Type: ir.TypeRef{Kind: ir.KindString}},
		},
		Indexes: []ir.Index{{Fields: []string{"company_id", "user_id"}, Unique: true}},
	}}
	if err := em.EmitSQL(entities); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "db", "schema", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	want := "CREATE UNIQUE INDEX IF NOT EXISTS uidx_memberships_companyid_userid ON memberships (companyid, userid);"
	if !strings.Contains(string(data), want) {
		t.Fatalf("missing composite unique index %q in:\n%s", want, data)
	}
}
