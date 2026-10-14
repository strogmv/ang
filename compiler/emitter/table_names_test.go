package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/compiler"
)

const tableNamesDomain = `package domain

Category: {
	id:       string @db(type="UUID", primary_key=true)
	parentId: string @db(type="UUID", index=true)
	name:     string @db(type="TEXT")
	indexes: [{fields: ["parentId", "name"], unique: true}]
}

Person: {
	@db_table(name="people")
	id:   string @db(type="UUID", primary_key=true)
	name: string @db(type="TEXT")
}
`

const tableNamesRepo = `package repo

Repositories: {
	Category: finders: [{name: "ListByParent", returns: "many", where: [{field: "parentId", op: "="}]}]
	Person: finders: [{name: "ListAll2", returns: "many"}]
}
`

// tableNamesSchema runs the pipeline over a project with Category and Person;
// project is the cue/project file, "" for none.
func tableNamesSchema(t *testing.T, project string) *ir.Schema {
	t.Helper()
	base := writeFinderProject(t, tableNamesRepo)
	if err := os.WriteFile(filepath.Join(base, "cue", "domain", "names.cue"), []byte(tableNamesDomain), 0o644); err != nil {
		t.Fatal(err)
	}
	if project != "" {
		if err := os.MkdirAll(filepath.Join(base, "cue", "project"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "cue", "project", "project.cue"), []byte(project), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := compiler.RunSemanticPhasesWithOptions(base, compiler.PipelineOptions{})
	if err != nil {
		t.Fatalf("semantic phases: %v", err)
	}
	irOut, err := compiler.RunIRPhase(compiler.IRPhaseInput{Normalized: out, BasePath: base})
	if err != nil {
		t.Fatalf("IR phase: %v", err)
	}
	return irOut.Schema
}

func emitTableNames(t *testing.T, schema *ir.Schema) (sql, categoryRepo, personRepo, conventions string) {
	t.Helper()
	em := New(t.TempDir(), t.TempDir(), "templates")
	em.GoModule = "example.com/app"
	if err := em.EmitSQL(schema.Entities); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitRepository(schema.Repos, schema.Entities); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitPostgresRepo(schema.Repos, schema.Entities); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitConventionsDoc(schema); err != nil {
		t.Fatal(err)
	}
	read := func(parts ...string) string {
		data, err := os.ReadFile(filepath.Join(append([]string{em.OutputDir}, parts...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	pg := []string{"internal", "adapter", "repository", "postgres"}
	return read("db", "schema", "schema.sql"),
		read(append(pg, "categoryrepository.go")...),
		read(append(pg, "personrepository.go")...),
		read("docs", "ang", "conventions.md")
}

func requireAll(t *testing.T, what, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("%s lacks %q", what, want)
		}
	}
}

func requireNone(t *testing.T, what, text string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(text, u) {
			t.Errorf("%s has %q", what, u)
		}
	}
}

func TestTableNamesPluralFromTarget(t *testing.T) {
	schema := tableNamesSchema(t, "package project\n\nstate: target: {lang: \"go\", table_names: \"plural\"}\n")
	sql, category, person, doc := emitTableNames(t, schema)
	requireAll(t, "schema.sql", sql,
		"CREATE TABLE IF NOT EXISTS categories (",
		"CREATE INDEX IF NOT EXISTS idx_categories_parentid ON categories (parentid);",
		"CREATE UNIQUE INDEX IF NOT EXISTS uidx_categories_parentid_name ON categories (parentid, name);",
		"CREATE TABLE IF NOT EXISTS people (",
	)
	requireNone(t, "schema.sql", sql, "categorys", "persons")
	requireAll(t, "category repository", category, "INSERT INTO categories", "FROM categories")
	requireNone(t, "category repository", category, "categorys")
	requireAll(t, "person repository", person, "INSERT INTO people", "FROM people")
	requireAll(t, "conventions.md", doc, "| `Category` | `categories` |", "| `Person` | `people` |")
}

func TestTableNamesLegacyByDefault(t *testing.T) {
	schema := tableNamesSchema(t, "")
	sql, category, person, doc := emitTableNames(t, schema)
	requireAll(t, "schema.sql", sql,
		"CREATE TABLE IF NOT EXISTS categorys (",
		"CREATE INDEX IF NOT EXISTS idx_categorys_parentid ON categorys (parentid);",
		"CREATE TABLE IF NOT EXISTS people (",
	)
	requireAll(t, "category repository", category, "FROM categorys")
	requireAll(t, "person repository", person, "FROM people")
	requireAll(t, "conventions.md", doc, "| `Category` | `categorys` |")
}

func TestTableNamesTargetValueChecked(t *testing.T) {
	base := writeFinderProject(t, "package repo\n")
	if err := os.MkdirAll(filepath.Join(base, "cue", "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "cue", "project", "project.cue"), []byte("package project\n\nstate: target: {lang: \"go\", table_names: \"plurals\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := compiler.RunSemanticPhasesWithOptions(base, compiler.PipelineOptions{})
	if err == nil || !strings.Contains(err.Error(), "table_names must be") {
		t.Fatalf("err = %v", err)
	}
}

func TestEmitSQLTableNameCollision(t *testing.T) {
	id := ir.Field{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}
	em := New(t.TempDir(), "", "templates")
	err := em.EmitSQL([]ir.Entity{
		{Name: "Person", Table: "people", Source: "a.cue:1", Fields: []ir.Field{id}},
		{Name: "People", Table: "people", Source: "b.cue:1", Fields: []ir.Field{id}},
	})
	if err == nil || !strings.Contains(err.Error(), "SQL_TABLE_NAME_COLLISION") {
		t.Fatalf("err = %v", err)
	}
}

func TestMongoCollectionUsesTableName(t *testing.T) {
	ent := ir.Entity{
		Name:     "Category",
		Table:    "categories",
		Metadata: map[string]any{"storage": "mongo"},
		Fields:   []ir.Field{{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "name", Type: ir.TypeRef{Kind: ir.KindString}}},
	}
	em := New(t.TempDir(), t.TempDir(), "templates")
	em.GoModule = "example.com/app"
	if err := em.EmitMongoRepo([]ir.Repository{{Name: "CategoryRepository", Entity: "Category"}}, []ir.Entity{ent}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(em.OutputDir, "internal", "adapter", "repository", "mongo", "categoryrepository.go"))
	if err != nil {
		t.Fatal(err)
	}
	requireAll(t, "mongo repository", string(data), `Collection("categories")`)
	requireNone(t, "mongo repository", string(data), "categorys")
}

func TestQueueSQLUsesTableName(t *testing.T) {
	ent := queueTestEntity(t, `@queue(handler="RunJob", key="companyId", unique="companyId,source")`)
	ent.Table = "job_runs"
	q := queueSQL(ent)
	if q.Table != "job_runs" || !strings.Contains(q.Claim, "FROM job_runs") {
		t.Fatalf("queue SQL = %#v", q)
	}
}
