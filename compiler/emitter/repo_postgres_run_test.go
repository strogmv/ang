package emitter

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// pgRunRepoCUE exercises the finder features against a real database.
const pgRunRepoCUE = `package repo

Repositories: {
	Product: {
		finders: [
			{
				name:        "UpsertBySku"
				kind:        "upsert"
				on_conflict: ["companyId", "sku"]
			},
			{
				name:    "ListBySkus"
				returns: "many"
				select:  "auto"
				params: [{name: "companyID", type: "uuid"}, {name: "skus", type: "[]string"}]
				sql: """
					SELECT p.* FROM products p WHERE p.companyid = $1 AND p.sku = ANY($2) ORDER BY p.sku
					"""
			},
			{
				name:    "PageByCompany"
				returns: "many"
				paged:   true
				params: [{name: "companyID", type: "uuid"}, {name: "since", type: "time"}]
				sql: """
					SELECT id::text, companyid::text, sku, name, price, createdat
					FROM products WHERE companyid = $1 AND createdat >= $2 ORDER BY sku LIMIT $3 OFFSET $4
					"""
			},
		]
	}
}
`

// pgRunMain drives the generated repository: a SaveBatch larger than one
// statement can bind, an upsert on the unique (companyid, sku) index, a
// select: "auto" finder with a []string parameter and a paged finder.
const pgRunMain = `package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"%[1]s/internal/adapter/repository/postgres"
	"%[1]s/internal/domain"
)

func must(err error) {
	if err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("ANG_RUN_DB"))
	must(err)
	defer pool.Close()
	schema, err := os.ReadFile(os.Getenv("ANG_RUN_SCHEMA"))
	must(err)
	_, err = pool.Exec(ctx, string(schema))
	must(err)

	repo := postgres.NewProductRepository(pool)
	company := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Second)

	// 6 columns per row: 10922 rows per statement, so 25000 take three.
	const total = 25000
	batch := make([]*domain.Product, total)
	for i := range batch {
		batch[i] = &domain.Product{ID: uuid.NewString(), CompanyID: company, Sku: fmt.Sprintf("sku-%%05d", i), Name: "first", Price: i, CreatedAt: now}
	}
	must(repo.SaveBatch(ctx, batch))
	var count int
	must(pool.QueryRow(ctx, "SELECT count(*) FROM products").Scan(&count))
	if count != total {
		must(fmt.Errorf("SaveBatch stored %%d rows, want %%d", count, total))
	}

	// A chunk that fails rolls the whole batch back.
	bad := make([]*domain.Product, total)
	for i := range bad {
		bad[i] = &domain.Product{ID: uuid.NewString(), CompanyID: company, Sku: fmt.Sprintf("new-%%05d", i), Name: "x", CreatedAt: now}
	}
	bad[total-1].ID = "not-a-uuid"
	if err := repo.SaveBatch(ctx, bad); err == nil {
		must(fmt.Errorf("SaveBatch with a bad row in the last chunk succeeded"))
	}
	must(pool.QueryRow(ctx, "SELECT count(*) FROM products").Scan(&count))
	if count != total {
		must(fmt.Errorf("a failed SaveBatch left %%d rows, want %%d", count, total))
	}

	// Upsert on (companyid, sku): new ids, same keys — rows keep their id.
	up := []*domain.Product{
		{ID: uuid.NewString(), CompanyID: company, Sku: "sku-00001", Name: "renamed", Price: 7, CreatedAt: now},
		{ID: uuid.NewString(), CompanyID: company, Sku: "sku-new", Name: "added", Price: 8, CreatedAt: now},
	}
	must(repo.UpsertBySku(ctx, up))
	must(pool.QueryRow(ctx, "SELECT count(*) FROM products").Scan(&count))
	if count != total+1 {
		must(fmt.Errorf("UpsertBySku left %%d rows, want %%d", count, total+1))
	}

	got, err := repo.ListBySkus(ctx, company, []string{"sku-00001", "sku-new"})
	must(err)
	if len(got) != 2 || got[0].Name != "renamed" || got[0].ID != batch[1].ID || got[1].Name != "added" || got[0].CreatedAt.IsZero() {
		must(fmt.Errorf("ListBySkus = %%+v", got))
	}

	page, err := repo.PageByCompany(ctx, company, now.Add(-time.Hour), 3, 2)
	must(err)
	if len(page) != 3 || page[0].Sku != "sku-00002" {
		must(fmt.Errorf("PageByCompany = %%+v", page))
	}
	fmt.Println("OK")
}
`

// Items 16 and 17 of docs/tech-debt-backlog.md, run for real: the generated
// repository compiled and run against Postgres. Needs ANG_TEST_POSTGRES_URL
// (a server where the test may create and drop angtest_* databases).
func TestGeneratedPostgresRepositoryRunsOnPostgres(t *testing.T) {
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
	db := fmt.Sprintf("angtest_repo_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+db); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE IF EXISTS "+db+" WITH (FORCE)")
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + db

	// The program must live inside this module to use its dependencies.
	name := fmt.Sprintf("pgrun_%d", time.Now().UnixNano())
	dir, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
		_ = os.Remove(filepath.Dir(dir)) // testdata, when nothing else is in it
	})
	module := "github.com/strogmv/ang/compiler/emitter/testdata/" + name

	schema := finderSchema(t, pgRunRepoCUE)
	em := New(dir, t.TempDir(), "templates")
	em.GoModule = module
	for _, step := range []func() error{
		func() error { return em.EmitDomain(schema.Entities) },
		func() error { return em.EmitPostgresRepo(schema.Repos, schema.Entities) },
		em.EmitPostgresCommon,
		func() error { return em.EmitSQL(schema.Entities) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	mainDir := filepath.Join(dir, "cmd", "run")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(fmt.Sprintf(pgRunMain, module)), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./testdata/"+name+"/cmd/run")
	cmd.Env = append(os.Environ(), "ANG_RUN_DB="+u.String(), "ANG_RUN_SCHEMA="+filepath.Join(dir, "db", "schema", "schema.sql"))
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "OK") {
		t.Fatalf("generated repository run failed: %v\n%s", err, out)
	}
	t.Logf("%s", out)
}
