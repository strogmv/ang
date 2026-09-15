package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

func TestEmitConventionsDocListsTablesAndDeleteFinders(t *testing.T) {
	root := t.TempDir()
	em := New(root, "", "templates")
	schema := &ir.Schema{
		Entities: []ir.Entity{
			{Name: "Country", Fields: []ir.Field{{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}}},
			{Name: "Money", Fields: []ir.Field{{Name: "amount", Type: ir.TypeRef{Kind: ir.KindInt}}}},
		},
		Repos: []ir.Repository{{Name: "CountryRepository", Entity: "Country", Finders: []ir.Finder{
			{Name: "DeleteExpired", Action: "delete"},
			{Name: "FindByCode", Action: "find", Returns: "one"},
		}}},
	}
	if err := em.EmitConventionsDoc(schema); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "ang", "conventions.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	for _, want := range []string{"| `Country` | `countrys` |", "- `CountryRepository.DeleteExpired` → `(int64, error)`", "W_RAW_BODY_IMPLICIT"} {
		if !strings.Contains(doc, want) {
			t.Errorf("conventions.md lacks %q:\n%s", want, doc)
		}
	}
	for _, unwanted := range []string{"`Money`", "FindByCode"} {
		if strings.Contains(doc, unwanted) {
			t.Errorf("conventions.md must not list %s:\n%s", unwanted, doc)
		}
	}
}
