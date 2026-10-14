package emitter

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/strogmv/ang/angir/ir"
)

// TestEmitSQLRunsOnPostgres applies a schema.sql that uses every index and
// constraint feature to a real database twice: it must be valid SQL and
// re-runnable. Set ANG_TEST_POSTGRES_URL to a server URL whose user may create
// databases (e.g. postgres://user:password@localhost:5439/postgres); the test
// creates angtest_<n> and drops it.
func TestEmitSQLRunsOnPostgres(t *testing.T) {
	base := os.Getenv("ANG_TEST_POSTGRES_URL")
	if base == "" {
		t.Skip("ANG_TEST_POSTGRES_URL not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	db := fmt.Sprintf("angtest_schema_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+db); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE IF EXISTS "+db+" WITH (FORCE)")
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + db
	conn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	ent := sqlTestEntity(
		ir.Index{Name: "uq_customers_open", Fields: []string{"companyId"}, Unique: true, Where: "email <> ''"},
		ir.Index{Name: "users_email_lower_key", Expr: "lower(email)", Unique: true},
		ir.Index{Name: "uq_customers_ext", Expr: "companyid, lower(btrim(email))", Unique: true, Where: "btrim(email) <> ''"},
		ir.Index{Fields: []string{"companyId", "email"}, Opclass: "text_pattern_ops"},
		ir.Index{Fields: []string{"email"}, Using: "hash", Concurrently: true},
	)
	ent.Checks = []ir.Check{{Name: "customers_email_valid", Expr: "email <> 'nobody'"}}
	ent.DDL = []string{`CREATE OR REPLACE FUNCTION customers_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'customers are append-only';
END
$$;
CREATE OR REPLACE TRIGGER customers_append_only BEFORE DELETE ON customers FOR EACH ROW EXECUTE FUNCTION customers_append_only();`}
	schema := emitSchemaSQL(t, New(t.TempDir(), "", "templates"), ent)
	for i := 0; i < 2; i++ {
		if _, err := conn.Exec(ctx, schema); err != nil {
			t.Fatalf("run %d: %v\n%s", i+1, err, schema)
		}
	}
	var indexes, checks, triggers int
	_ = conn.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE tablename = 'customers'`).Scan(&indexes)
	_ = conn.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conname = 'customers_email_valid'`).Scan(&checks)
	_ = conn.QueryRow(ctx, `SELECT count(*) FROM pg_trigger WHERE tgname = 'customers_append_only'`).Scan(&triggers)
	if indexes != 6 || checks != 1 || triggers != 1 { // 5 declared + the primary key
		t.Fatalf("indexes=%d checks=%d triggers=%d", indexes, checks, triggers)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO customers (id, companyid, email, tags) VALUES ('1', 'c', 'nobody', 't')`); err == nil {
		t.Fatal("the check did not reject a row")
	}
}
