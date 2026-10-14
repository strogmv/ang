# Repositories: finders, hand-written SQL, batches, transactions

`ang build` gives every PostgreSQL entity a repository: the port
`internal/port/<entity>repository.go` and its implementation
`internal/adapter/repository/postgres/<entity>repository.go`. The built-in
methods are `Save`, `SaveBatch`, `FindByID`, `Delete`, `Insert`, `Update`,
`LockByID` and `ListAll`. The finders of `cue/repo` add the rest:

```cue
package repo

Repositories: {
	Product: {
		finders: [
			{name: "ListByCompany", returns: "many", where: [{field: "companyId", op: "="}], order_by: "sku"},
		]
	}
}
```

A finder without `sql:` writes its own `SELECT … FROM <table> WHERE … ORDER BY …`;
its parameters come from `where:`. This page is about everything else.

## Parameters of a `sql:` finder: `params:`

Hand-written SQL declares its method parameters with `params:`, bound to
`$1, $2, …` in the order written:

```cue
{
	name:    "ListChangedByIDs"
	returns: "many"
	params: [
		{name: "companyID", type: "uuid"},
		{name: "ids",       type: "[]uuid"},
		{name: "since",     type: "time"},
	]
	sql: """
		SELECT id::text, companyid::text, sku, name, price, createdat
		FROM products
		WHERE companyid = $1 AND id = ANY($2::uuid[]) AND createdat >= $3
		"""
}
// ListChangedByIDs(ctx context.Context, companyID string, ids []string, since time.Time) ([]domain.Product, error)
```

| `type` | Go | | `type` | Go |
|---|---|---|---|---|
| `string`, `text`, `uuid` | `string` | | `time` | `time.Time` |
| `int` | `int` | | `[]string`, `[]uuid` | `[]string` |
| `int64` | `int64` | | `[]int` | `[]int` |
| `float` | `float64` | | `[]int64` | `[]int64` |
| `bool` | `bool` | | | |

A uuid travels as text: compare it with a uuid column directly or cast it
(`$1::uuid`, `$2::uuid[]`). `params:` belongs to `sql:` finders only and
replaces `where:` there — the two together are an error, as are an unknown
type, a name that is not a Go identifier and a name declared twice.

The older form — fake `where:` entries whose `field` only exists to name a
parameter — keeps working. An explicit `param_type` is kept as written; only
a `where:` entry without one takes the type of its entity field.

## Paging: `paged: true`

`paged: true` appends `(limit int, offset int)` after the other parameters.
A finder without `sql:` writes `LIMIT $n+1 OFFSET $n+2` itself; a `sql:`
finder uses them in its SQL, where n is the number of the other parameters:

```cue
{
	name:    "PageByCompany"
	returns: "many"
	paged:   true
	params: [{name: "companyID", type: "uuid"}]
	sql: "SELECT … FROM products WHERE companyid = $1 ORDER BY sku LIMIT $2 OFFSET $3"
}
// PageByCompany(ctx context.Context, companyID string, limit int, offset int) ([]domain.Product, error)
```

Without `paged:` the legacy rule applies: `limit: N` with any N > 0, or a
finder name containing `paginate`, adds the same two parameters — `limit: 50`
does not limit the query to 50 rows. Each such finder gets the warning
`REPO_FINDER_PAGED_IMPLICIT`; `paged: true` says the same thing plainly and
generates the same code. `paged: false` turns the legacy rule off.

## Checks of `sql:` at generate time

`ang validate` and `ang build` read every `sql:` (comments, strings,
dollar-quoted bodies skipped) and fail with `REPO_FINDER_INVALID` and the
finder's `file:line` when the SQL and the generated method disagree:

- **Parameters.** The SQL must use exactly `$1 … $n` for the n arguments the
  method passes (`where:` or `params:`, then limit and offset). A `$3` with
  two arguments, or an argument the SQL never uses, would fail at run time
  ("expected 2 arguments, got 3", "could not determine data type of
  parameter $2").
- **Result columns.** The select list (after any `WITH`; the `RETURNING`
  list of an `INSERT`/`UPDATE`/`DELETE`) must have as many columns as the
  method scans, in the same order. A finder returning the entity scans its
  stored fields in CUE order; one with `return_type` and `scan_fields` scans
  those, in that order; a scalar return (`count`, `[]string`, …) scans one
  column. When a column's name or alias is a field scanned at another
  position, the error names the swap.
- **Casts.** pgx v5 refuses to scan `timestamp`, `timestamptz`, `date`,
  `time`, `bool`, `inet`, `cidr` and `interval` into a Go `string` (uuid,
  numeric, json and text scan fine). A plain column of such a type scanned
  into a string field needs `::text`.

A select list ANG cannot read for certain (`SELECT *`, `VALUES`, a statement
that starts with a parenthesis, an `UPDATE` without `RETURNING`) is not
checked.

## `select: "auto"`

With `select: "auto"` the SQL selects `*` or `alias.*` and ANG writes the
columns the method scans, in scan order and with the same casts as its own
finders (a non-text string column gets `::text`):

```cue
{
	name:    "ListBySkus"
	returns: "many"
	select:  "auto"
	params: [{name: "companyID", type: "uuid"}, {name: "skus", type: "[]string"}]
	sql: "SELECT p.* FROM products p WHERE p.companyid = $1 AND p.sku = ANY($2) ORDER BY p.sku"
}
// SELECT p.id::text, p.companyid::text, p.sku, p.name, p.price, p.createdat FROM products p …
```

It needs a finder that returns the entity, or a `return_type` with
`scan_fields`, and a select list that is exactly one `*`.

## `SaveBatch` and upserts

`SaveBatch(ctx, entities)` writes one multi-row `INSERT … ON CONFLICT (id)
DO UPDATE`. Postgres binds at most 65535 parameters per statement, so a
batch of more than `65535 / columns` entities is written in chunks inside
one transaction — the one in ctx when there is one, otherwise its own — and
a failing chunk rolls the whole batch back. A batch that fits one statement
is the single statement it always was.

`kind: "upsert"` writes on a unique index other than the id:

```cue
// cue/domain: indexes: [{fields: ["companyId", "sku"], unique: true}]
{name: "UpsertBySku", kind: "upsert", on_conflict: ["companyId", "sku"]}
// UpsertBySku(ctx context.Context, entities []*domain.Product) error
// INSERT … ON CONFLICT (companyid, sku) DO UPDATE SET <every other column but id>
```

`on_conflict` must be the fields of a unique, non-expression index of the
entity (or `["id"]`); a partial index repeats its `where` in the conflict
target. Anything else fails at generate time (Postgres would refuse it at
run time). A row that already exists keeps its id; one batch must not hold
the same key twice (Postgres: "cannot affect row a second time"). Upserts
chunk like `SaveBatch`. `onConflict:` is accepted as a spelling of
`on_conflict:`.

## Transactions from an operation

Three ways, all on the generated `TxManager` (one transaction per request;
a nested call joins the outer one as a savepoint):

- **`transactional: true`** on an operation runs all of it in one
  transaction. A flow becomes the body of one `tx.Block`; `impls: go:` code
  runs inside `s.txManager.WithTx`. An error rolls back; `flow.Return`
  commits and returns. Not for streaming operations.

  ```cue
  MarkInvoicePaid: {
  	service:       "billing"
  	transactional: true
  	flow: [ … ]
  }
  ```

- **`tx.Block`** in a flow wraps some of its steps.
- **`s.Tx.WithTx(ctx, func(ctx context.Context) error { … })`** in raw Go.
  `s.Tx` is the service's TxManager; any service whose Go mentions it gets
  the field (the older private name `s.txManager` works too). Use the `ctx`
  of the closure for every repository call.

Projects keep their own copy of `cue/schema/types.cue`: add
`transactional?: bool` to `#Operation` there to use the key under the
schema.
