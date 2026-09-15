package emitter

import (
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

// Generated repository ports must already be gofmt'ed, so gofmt -l on a
// generated project lists only real problems.
func TestEmitRepositoryWritesGofmtedPorts(t *testing.T) {
	root := t.TempDir()
	em := New(root, "", "templates")
	em.GoModule = "example.com/test"
	entities := []ir.Entity{{
		Name: "User",
		Fields: []ir.Field{
			{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
			{Name: "email", Type: ir.TypeRef{Kind: ir.KindString}},
		},
	}}
	repos := []ir.Repository{{
		Name:   "UserRepository",
		Entity: "User",
		Finders: []ir.Finder{
			{Name: "FindByEmail", Action: "find", Returns: "one", Where: []ir.WhereClause{{Field: "email", Param: "email", ParamType: "string"}}},
			{Name: "DeleteExpired", Action: "delete"},
		},
	}}
	if err := em.EmitRepository(repos, entities); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "internal", "port", "userrepository.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(data)
	if err != nil {
		t.Fatalf("generated port does not parse: %v\n%s", err, data)
	}
	if string(formatted) != string(data) {
		t.Fatalf("generated port is not gofmt'ed:\n%s", data)
	}
}
