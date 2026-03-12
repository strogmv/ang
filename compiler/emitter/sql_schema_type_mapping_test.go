package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitSQL_OverridesFallbackTextForTypedFields(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "sdk", "templates")

	entities := []ir.Entity{{
		Name: "Sample",
		Fields: []ir.Field{
			{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}, Metadata: map[string]any{"sql_type": "TEXT"}, Attributes: []ir.Attribute{{Name: "db", Args: map[string]any{"type": "TEXT"}}}},
			{Name: "count", Type: ir.TypeRef{Kind: ir.KindInt}, Metadata: map[string]any{"sql_type": "TEXT"}, Attributes: []ir.Attribute{{Name: "db", Args: map[string]any{"type": "TEXT"}}}},
			{Name: "enabled", Type: ir.TypeRef{Kind: ir.KindBool}, Metadata: map[string]any{"sql_type": "TEXT"}, Attributes: []ir.Attribute{{Name: "db", Args: map[string]any{"type": "TEXT"}}}},
			{Name: "createdAt", Type: ir.TypeRef{Kind: ir.KindTime}, Metadata: map[string]any{"sql_type": "TEXT"}, Attributes: []ir.Attribute{{Name: "db", Args: map[string]any{"type": "TEXT"}}}},
		},
	}}

	if err := em.EmitSQL(entities); err != nil {
		t.Fatalf("EmitSQL: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "db", "schema", "schema.sql"))
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}
	schema := string(data)

	if !strings.Contains(schema, `"count" BIGINT`) {
		t.Fatalf("expected BIGINT for int field, got schema:\n%s", schema)
	}
	if !strings.Contains(schema, `"enabled" BOOLEAN`) {
		t.Fatalf("expected BOOLEAN for bool field, got schema:\n%s", schema)
	}
	if !strings.Contains(schema, `"createdat" TIMESTAMPTZ`) {
		t.Fatalf("expected TIMESTAMPTZ for time field, got schema:\n%s", schema)
	}
}

func TestEmitSQL_AssignsPrimaryKeyForIDWhenMissing(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "sdk", "templates")

	entities := []ir.Entity{{
		Name: "Project",
		Fields: []ir.Field{
			{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
			{Name: "name", Type: ir.TypeRef{Kind: ir.KindString}},
		},
	}}

	if err := em.EmitSQL(entities); err != nil {
		t.Fatalf("EmitSQL: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "db", "schema", "schema.sql"))
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}
	schema := string(data)

	if !strings.Contains(schema, `"id" TEXT PRIMARY KEY NOT NULL`) {
		t.Fatalf("expected id PRIMARY KEY, got schema:\n%s", schema)
	}
}
