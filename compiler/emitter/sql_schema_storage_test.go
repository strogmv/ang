package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

// A MongoDB entity and a DTO have no PostgreSQL table; a plain entity does.
func TestEmitSQLSkipsMongoAndDTOEntities(t *testing.T) {
	root := t.TempDir()
	em := New(root, "", "templates")
	id := ir.Field{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}
	entities := []ir.Entity{
		{Name: "Order", Fields: []ir.Field{id}},
		{Name: "ChatMessage", Fields: []ir.Field{id}, Metadata: map[string]any{"storage": "mongo"}},
		{Name: "DocView", Fields: []ir.Field{id}, Metadata: map[string]any{"dto": true}},
	}
	if err := em.EmitSQL(entities); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "db", "schema", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	schema := string(data)
	if !strings.Contains(schema, "orders") {
		t.Fatalf("no table for a plain entity:\n%s", schema)
	}
	for _, table := range []string{"chatmessages", "docviews"} {
		if strings.Contains(schema, table) {
			t.Errorf("table %s emitted for an entity that is not stored in PostgreSQL", table)
		}
	}
}
