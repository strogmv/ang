package emitter

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// generatedSystemSQL emits systemrepository.go and returns every SQL string it
// passes to Exec, Query or QueryRow, keyed by the repository method.
func generatedSystemSQL(t *testing.T) map[string][]string {
	t.Helper()
	root := t.TempDir()
	em := New(root, "", "templates")
	em.GoModule = "example.com/app"
	if err := em.EmitSystemRepository(); err != nil {
		t.Fatalf("EmitSystemRepository: %v", err)
	}
	path := filepath.Join(root, "internal", "adapter", "repository", "postgres", "systemrepository.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse generated repository: %v", err)
	}
	out := map[string][]string{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			switch sel.Sel.Name {
			case "Exec", "Query", "QueryRow":
			default:
				return true
			}
			lit, ok := call.Args[1].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Errorf("%s: SQL is not a string literal", fn.Name.Name)
				return true
			}
			sql, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatalf("%s: %v", fn.Name.Name, err)
			}
			out[fn.Name.Name] = append(out[fn.Name.Name], sql)
			return true
		})
	}
	return out
}

var (
	ddlTableRe = regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS (\w+) \((.*?)\n\);`)
	ddlIndexRe = regexp.MustCompile(`CREATE (?:UNIQUE )?INDEX IF NOT EXISTS \w+ ON (\w+) \(([^)]*)\)(?: WHERE (\w+))?`)
	sqlTableRe = regexp.MustCompile(`(?i)\b(?:FROM|INTO|UPDATE)\s+(\w+)`)
	sqlWordRe  = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

// systemTableColumns parses systemTablesDDL into table -> set of columns.
func systemTableColumns(t *testing.T) map[string]map[string]bool {
	t.Helper()
	tables := map[string]map[string]bool{}
	for _, m := range ddlTableRe.FindAllStringSubmatch(systemTablesDDL, -1) {
		cols := map[string]bool{}
		for _, line := range strings.Split(m[2], "\n") {
			if f := strings.Fields(line); len(f) > 0 {
				cols[f[0]] = true
			}
		}
		tables[m[1]] = cols
	}
	if len(tables) != 3 {
		t.Fatalf("expected 3 system tables in systemTablesDDL, got %v", tables)
	}
	for _, m := range ddlIndexRe.FindAllStringSubmatch(systemTablesDDL, -1) {
		cols, ok := tables[m[1]]
		if !ok {
			t.Errorf("index on unknown table %s", m[1])
			continue
		}
		for _, c := range append(strings.Split(m[2], ","), m[3]) {
			if c = strings.TrimSpace(c); c != "" && !cols[c] {
				t.Errorf("index on %s uses column %s that the table does not declare", m[1], c)
			}
		}
	}
	return tables
}

var sqlKeywords = map[string]bool{
	"select": true, "from": true, "where": true, "insert": true, "into": true, "values": true,
	"on": true, "conflict": true, "do": true, "update": true, "set": true, "is": true, "null": true,
	"not": true, "and": true, "or": true, "order": true, "by": true, "limit": true, "delete": true,
	"now": true, "text": true, "asc": true, "desc": true,
}

// TestSystemRepositorySQLMatchesSystemTables: every column the generated
// SystemRepository names exists in the DDL that schema.sql gets (backlog #6:
// the repository wrote outbox_events.topic and idempotency_keys.response,
// neither of which the DDL declared).
func TestSystemRepositorySQLMatchesSystemTables(t *testing.T) {
	tables := systemTableColumns(t)
	stmts := generatedSystemSQL(t)
	if len(stmts) == 0 {
		t.Fatal("no SQL found in the generated SystemRepository")
	}
	for method, list := range stmts {
		for _, sql := range list {
			tm := sqlTableRe.FindStringSubmatch(sql)
			if tm == nil {
				t.Errorf("%s: no table in %q", method, sql)
				continue
			}
			cols, ok := tables[tm[1]]
			if !ok {
				t.Errorf("%s: table %s is not in systemTablesDDL", method, tm[1])
				continue
			}
			for _, w := range sqlWordRe.FindAllString(sql, -1) {
				if w == tm[1] || sqlKeywords[strings.ToLower(w)] {
					continue
				}
				if !cols[w] {
					t.Errorf("%s: column %s.%s is not declared in systemTablesDDL (%q)", method, tm[1], w, sql)
				}
			}
		}
	}

	pending := strings.Join(stmts["ListPending"], "\n")
	if !strings.Contains(pending, "ORDER BY sequence_no") {
		t.Errorf("ListPending must deliver in insert order (ORDER BY sequence_no), got %q", pending)
	}
	if !strings.Contains(strings.Join(stmts["SaveEvent"], "\n"), "(id, topic, payload)") {
		t.Errorf("SaveEvent must write id, topic, payload: %v", stmts["SaveEvent"])
	}
}

func TestEmitSQLWritesSystemTables(t *testing.T) {
	root := t.TempDir()
	em := New(root, "", "templates")
	if err := em.EmitSQL(nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "db", "schema", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), systemTablesDDL) {
		t.Fatalf("schema.sql does not start with systemTablesDDL:\n%s", data)
	}
}

// TestSystemRepositorySQLRunsOnPostgres executes the generated statements on
// a real PostgreSQL built from systemTablesDDL. It needs ANG_TEST_PG_URL, a
// URL of a server where the user may create databases (for example
// postgres://user:password@localhost:5439/postgres?sslmode=disable); it creates
// a database named angtest_* and drops it afterwards.
func TestSystemRepositorySQLRunsOnPostgres(t *testing.T) {
	adminURL := os.Getenv("ANG_TEST_PG_URL")
	if adminURL == "" {
		t.Skip("ANG_TEST_PG_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	dbName := fmt.Sprintf("angtest_system_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		admin.Close(context.Background())
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		defer admin.Close(context.Background())
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)"); err != nil {
			t.Errorf("drop database %s: %v", dbName, err)
		}
	})

	cfg, err := pgx.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Database = dbName
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect %s: %v", dbName, err)
	}
	defer conn.Close(context.Background())

	// schema.sql is applied twice in a row by some setups: it must be re-runnable.
	for i := 0; i < 2; i++ {
		if _, err := conn.PgConn().Exec(ctx, systemTablesDDL).ReadAll(); err != nil {
			t.Fatalf("apply systemTablesDDL (run %d): %v", i+1, err)
		}
	}

	// Every generated statement must at least prepare: unknown columns fail here.
	for method, list := range generatedSystemSQL(t) {
		for _, sql := range list {
			if _, err := conn.Prepare(ctx, "", sql); err != nil {
				t.Errorf("%s: %q does not prepare against systemTablesDDL: %v", method, sql, err)
			}
		}
	}
	if t.Failed() {
		return
	}

	stmts := generatedSystemSQL(t)
	saveEvent, listPending, markProcessed := stmts["SaveEvent"][0], stmts["ListPending"][0], stmts["MarkProcessed"][0]

	// Random UUIDs inserted in a known order come back in that order.
	ids := []string{
		"f0000000-0000-4000-8000-000000000001",
		"10000000-0000-4000-8000-000000000002",
		"a0000000-0000-4000-8000-000000000003",
	}
	for i, id := range ids {
		if _, err := conn.Exec(ctx, saveEvent, id, fmt.Sprintf("topic.%d", i), []byte(`{"n":`+strconv.Itoa(i)+`}`)); err != nil {
			t.Fatalf("SaveEvent: %v", err)
		}
	}
	readPending := func() []string {
		rows, err := conn.Query(ctx, listPending, 10)
		if err != nil {
			t.Fatalf("ListPending: %v", err)
		}
		defer rows.Close()
		var got []string
		for rows.Next() {
			var id, topic string
			var payload []byte
			if err := rows.Scan(&id, &topic, &payload); err != nil {
				t.Fatalf("scan: %v", err)
			}
			got = append(got, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := readPending(); strings.Join(got, ",") != strings.Join(ids, ",") {
		t.Fatalf("ListPending order = %v, want insert order %v", got, ids)
	}
	tag, err := conn.Exec(ctx, markProcessed, ids[1])
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("MarkProcessed: rows=%d err=%v", tag.RowsAffected(), err)
	}
	if tag, err := conn.Exec(ctx, markProcessed, ids[1]); err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("MarkProcessed twice: rows=%d err=%v", tag.RowsAffected(), err)
	}
	if got := readPending(); strings.Join(got, ",") != ids[0]+","+ids[2] {
		t.Fatalf("ListPending after MarkProcessed = %v", got)
	}

	// Idempotency keys round-trip.
	save, check := stmts["Save"][0], stmts["Check"][0]
	for _, body := range []string{`{"ok":true}`, `{"ok":false}`} {
		if _, err := conn.Exec(ctx, save, "k1", []byte(body)); err != nil {
			t.Fatalf("idempotency Save: %v", err)
		}
	}
	var stored []byte
	if err := conn.QueryRow(ctx, check, "k1").Scan(&stored); err != nil {
		t.Fatalf("idempotency Check: %v", err)
	}
	if string(stored) != `{"ok": false}` && string(stored) != `{"ok":false}` {
		t.Fatalf("idempotency Check returned %s", stored)
	}
}
