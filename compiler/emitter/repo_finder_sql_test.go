package emitter

import (
	"strings"
	"testing"
)

// Item 14 of docs/tech-debt-backlog.md: a sql: finder declares its method
// parameters with params: instead of fake where: fields.
func TestFinderParamsDeclareTheMethod(t *testing.T) {
	schema := finderSchema(t, `package repo

Repositories: {
	Product: {
		finders: [{
			name:    "ListChangedByIDs"
			returns: "many"
			params: [
				{name: "companyID", type: "uuid"},
				{name: "ids", type: "[]uuid"},
				{name: "since", type: "time"},
				{name: "minPrice", type: "int64"},
				{name: "active", type: "bool"},
			]
			sql: """
				SELECT id::text, companyid::text, sku, name, price, createdat
				FROM products
				WHERE companyid = $1 AND id = ANY($2::uuid[]) AND createdat >= $3 AND price >= $4 AND $5
				"""
		}]
	}
}
`)
	port, repo := emitProductRepo(t, schema)
	wantSig := "ListChangedByIDs(ctx context.Context, companyID string, ids []string, since time.Time, minPrice int64, active bool) ([]domain.Product, error)"
	if !strings.Contains(port, wantSig) {
		t.Fatalf("port lacks %q:\n%s", wantSig, port)
	}
	if !strings.Contains(repo, "ListChangedByIDs(ctx context.Context, companyID string, ids []string, since time.Time, minPrice int64, active bool)") {
		t.Fatalf("postgres repository lacks the parameters:\n%s", repo)
	}
	if !strings.Contains(repo, "createdat >= $3 AND price >= $4 AND $5`, companyID, ids, since, minPrice, active)") {
		t.Fatalf("postgres repository does not pass the parameters in order:\n%s", repo)
	}
}

// Item 15: paged: true appends (limit int, offset int) after the declared
// parameters; the sql uses them as $n+1 and $n+2.
func TestPagedFinderTakesLimitAndOffset(t *testing.T) {
	schema := finderSchema(t, `package repo

Repositories: {
	Product: {
		finders: [
			{
				name:    "PageByCompany"
				returns: "many"
				paged:   true
				params: [{name: "companyID", type: "uuid"}]
				sql: """
					SELECT id::text, companyid::text, sku, name, price, createdat
					FROM products WHERE companyid = $1 ORDER BY sku LIMIT $2 OFFSET $3
					"""
			},
			{
				name:    "ListByCompany"
				returns: "many"
				paged:   true
				where: [{field: "companyId", op: "=", param: "companyID"}]
				order_by: "sku"
			},
			{
				// The legacy rule — a name containing "paginate" — is off.
				name:    "PaginateAll"
				returns: "many"
				paged:   false
			},
		]
	}
}
`)
	port, repo := emitProductRepo(t, schema)
	for _, want := range []string{
		"PageByCompany(ctx context.Context, companyID string, limit int, offset int) ([]domain.Product, error)",
		"ListByCompany(ctx context.Context, companyID string, limit int, offset int) ([]domain.Product, error)",
		"PaginateAll(ctx context.Context) ([]domain.Product, error)",
	} {
		if !strings.Contains(port, want) {
			t.Errorf("port lacks %q:\n%s", want, port)
		}
	}
	if !strings.Contains(repo, "LIMIT $2 OFFSET $3`, companyID, limit, offset)") {
		t.Errorf("PageByCompany does not pass limit and offset:\n%s", repo)
	}
	if !strings.Contains(repo, "ORDER BY sku\n\t\tLIMIT $2 OFFSET $3\n\t`, companyID, limit, offset)") {
		t.Errorf("ListByCompany does not write LIMIT/OFFSET:\n%s", repo)
	}
}

// Item 15: an argument-count mismatch is a generate-time error with the
// finder's file:line, not a "too many arguments" at run time.
func TestFinderArgumentMismatchFailsValidation(t *testing.T) {
	cases := []struct {
		name  string
		cue   string
		wants []string
	}{
		{
			name: "placeholder without parameter",
			cue: `{
				name:    "ByCompany"
				returns: "many"
				params: [{name: "companyID", type: "uuid"}]
				sql: "SELECT id, companyid, sku, name, price, createdat FROM products WHERE companyid = $1 LIMIT $2"
			}`,
			wants: []string{"products.cue:5:", "Product.ByCompany", "sql uses $2", "$1=companyID", "paged: true"},
		},
		{
			name: "parameter the sql does not use",
			cue: `{
				name:    "ByCompany"
				returns: "many"
				paged:   true
				params: [{name: "companyID", type: "uuid"}]
				sql: "SELECT id, companyid, sku, name, price, createdat FROM products WHERE companyid = $1"
			}`,
			wants: []string{"sql never uses $2 (limit), $3 (offset)"},
		},
		{
			name: "legacy where fields counted too",
			cue: `{
				name:    "ByCompany"
				returns: "count"
				where: [{field: "companyId", op: "=", param: "companyID"}, {field: "sku", op: "=", param: "sku"}]
				sql: "SELECT count(*) FROM products WHERE companyid = $1"
			}`,
			wants: []string{"sql never uses $2 (sku)"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := finderValidationError(t, "package repo\n\nRepositories: {\n\tProduct: {\n\t\tfinders: ["+tc.cue+"]\n\t}\n}\n")
			requireFinderError(t, err, tc.wants...)
		})
	}
}

// Item 16: the select list of a sql: finder must match what the method
// scans — count, order, and the ::text a timestamp needs to land in a string.
func TestFinderSelectListMustMatchTheScan(t *testing.T) {
	cases := []struct {
		name  string
		cue   string
		wants []string
	}{
		{
			name: "missing column",
			cue: `{
				name: "ByCompany", returns: "many", params: [{name: "companyID", type: "uuid"}]
				sql: "SELECT id::text, companyid::text, sku, name, price FROM products WHERE companyid = $1"
			}`,
			wants: []string{"sql returns 5 columns, but the method scans 6 into domain.Product", "id, companyid, sku, name, price, createdat"},
		},
		{
			name: "columns swapped",
			cue: `{
				name: "ByCompany", returns: "many", params: [{name: "companyID", type: "uuid"}]
				sql: "SELECT id::text, companyid::text, name, sku, price, createdat FROM products p WHERE companyid = $1"
			}`,
			wants: []string{"column 3 of the sql is name, but the method scans sku there and name at position 4"},
		},
		{
			name: "scalar finder returns two columns",
			cue: `{
				name: "CountByCompany", returns: "count", params: [{name: "companyID", type: "uuid"}]
				sql: "SELECT count(*), max(price) FROM products WHERE companyid = $1"
			}`,
			wants: []string{"sql returns 2 columns, but the method returns int64 and scans exactly one"},
		},
		{
			name: "date into a string",
			cue: `{
				name: "Names", returns: "[]domain.ProductStamp", return_type: "[]domain.ProductStamp"
				scan_fields: ["name", "stamp"]
				sql: "SELECT name, stamp FROM products"
			}`,
			wants: []string{"column 2 (stamp) is date, which pgx cannot scan into a Go string; write stamp::text"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := "package repo\n\nRepositories: {\n\tProduct: {\n\t\tfinders: [" + tc.cue + "]\n\t}\n}\n"
			requireFinderError(t, finderValidationError(t, repo), tc.wants...)
		})
	}
}

// Item 16: select: "auto" makes ANG write the select list (with the casts
// of the generated finders) in place of * or alias.*.
func TestSelectAutoWritesTheColumnList(t *testing.T) {
	schema := finderSchema(t, `package repo

Repositories: {
	Product: {
		finders: [{
			name:    "ListBySkus"
			returns: "many"
			select:  "auto"
			params: [{name: "companyID", type: "uuid"}, {name: "skus", type: "[]string"}]
			sql: """
				SELECT p.* FROM products p JOIN unnest($2::text[]) s(sku) ON s.sku = p.sku WHERE p.companyid = $1
				"""
		}]
	}
}
`)
	_, repo := emitProductRepo(t, schema)
	want := "SELECT p.id::text, p.companyid::text, p.sku, p.name, p.price, p.createdat FROM products p JOIN unnest($2::text[])"
	if !strings.Contains(repo, want) {
		t.Fatalf("select: auto did not write %q:\n%s", want, repo)
	}

	err := finderValidationError(t, `package repo

Repositories: {
	Product: {
		finders: [{
			name: "Bad", returns: "many", select: "auto"
			sql: "SELECT id, sku FROM products"
		}]
	}
}
`)
	requireFinderError(t, err, `select: "auto": the select list must be exactly * or alias.*`)
}

// dealingi-back's finders use explicit casts and fake where fields; a
// legacy finder that matches its SQL passes the checks unchanged.
func TestLegacyFinderThatMatchesItsSQLPasses(t *testing.T) {
	err := finderValidationError(t, `package repo

Repositories: {
	Product: {
		finders: [{
			name:    "Search"
			returns: "many"
			limit:   50
			where: [
				{field: "companyId", op: "=", param: "companyID"},
				{field: "name", op: "=", param: "pattern", param_type: "string"},
			]
			sql: """
				SELECT id::text, companyid::text, sku, name, price, createdat FROM products
				WHERE companyid = $1::uuid AND name ILIKE '%' || $2 || '%' -- not $9
				ORDER BY sku LIMIT $3 OFFSET $4
				"""
		}]
	}
}
`)
	if err != nil {
		t.Fatalf("legacy finder rejected: %v", err)
	}
}

// Item 17: SaveBatch writes a batch of any size (chunks below the 65535
// parameters of one statement, in one transaction); kind: "upsert" writes on
// a unique index other than the id.
func TestSaveBatchChunksAndUpsertOnUniqueIndex(t *testing.T) {
	schema := finderSchema(t, `package repo

Repositories: {
	Product: {
		finders: [{name: "UpsertBySku", kind: "upsert", on_conflict: ["sku", "companyId"]}]
	}
}
`)
	port, repo := emitProductRepo(t, schema)
	if !strings.Contains(port, "UpsertBySku(ctx context.Context, entities []*domain.Product) error") {
		t.Fatalf("port lacks UpsertBySku:\n%s", port)
	}
	if strings.Contains(repo, "batch is too large") {
		t.Fatalf("SaveBatch still refuses a large batch:\n%s", repo)
	}
	for _, want := range []string{
		"const maxBatchRows = 65535 / columnsPerEntity",
		"return NewTxManager(r.DB).WithTx(ctx, func(ctx context.Context) error {",
		"ON CONFLICT (sku, companyid) DO UPDATE\n\t\tSET name = EXCLUDED.name, price = EXCLUDED.price, createdat = EXCLUDED.createdat`)",
		"ON CONFLICT (id) DO UPDATE\n\t\tSET companyid = EXCLUDED.companyid, sku = EXCLUDED.sku",
	} {
		if !strings.Contains(repo, want) {
			t.Errorf("repository lacks %q:\n%s", want, repo)
		}
	}

	err := finderValidationError(t, `package repo

Repositories: {
	Product: {
		finders: [{name: "UpsertByName", kind: "upsert", on_conflict: ["companyId", "name"]}]
	}
}
`)
	requireFinderError(t, err, "products.cue:5:", "on_conflict: (companyid, name) is not the column list of a unique index of Product")

	err = finderValidationError(t, `package repo

Repositories: {
	Product: {
		finders: [{name: "SaveBatch", kind: "upsert", on_conflict: ["id"]}]
	}
}
`)
	requireFinderError(t, err, "the repository already has a method SaveBatch")
}
