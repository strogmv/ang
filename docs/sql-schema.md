# SQL schema: indexes, checks, raw DDL, migrations

`ang build` writes `db/schema/schema.sql` from the domain entities: system
tables, then for each PostgreSQL entity its `CREATE TABLE IF NOT EXISTS`, its
indexes, its CHECK constraints and its raw DDL. The file is re-runnable: every
statement is idempotent. Entities stored in MongoDB and DTOs get no table.

## Indexes

A field index: `@db(index=true)` → `idx_<table>_<column>`. Everything else goes
into the entity's `indexes: [...]` list (`schema.#Index` in `cue/schema/sql.cue`
describes it; ANG rejects an unknown key such as `wher:` with file:line):

```cue
Customer: {
	id:        string @db(type="UUID", primary_key=true)
	companyId: string @db(type="UUID")
	email:     string @db(type="TEXT")
	cartToken?: string @db(type="TEXT")
	tags: [...string] @db(type="TEXT[]")
	indexes: [
		// composite, generated name uidx_customers_companyid_email
		{fields: ["companyId", "email"], unique: true},
		// explicit name
		{name: "ix_customers_company", fields: ["companyId"]},
		// partial
		{name: "uq_customers_carttoken", fields: ["cartToken"], unique: true, where: "carttoken IS NOT NULL AND carttoken <> ''"},
		// access method
		{name: "idx_customers_tags", fields: ["tags"], using: "gin"},
		// expression (needs name; fields and expr exclude each other)
		{name: "uq_customers_email_lower", expr: "companyid, lower(email)", unique: true},
		// operator class for every column in fields
		{fields: ["email"], opclass: "text_pattern_ops"},
		// built CONCURRENTLY by migrations from ang migrate diff
		{name: "idx_customers_email_big", fields: ["email"], concurrently: true},
	]
}
```

| key | meaning |
|---|---|
| `fields` | entity field names, the columns |
| `expr` | raw key list of an expression index, replaces `fields` |
| `name` | index name: lower-case identifier, at most 63 bytes |
| `unique` | `CREATE UNIQUE INDEX` |
| `where` | predicate of a partial index |
| `using` | `btree`, `hash`, `gist`, `spgist`, `gin`, `brin` |
| `opclass` | operator class appended to each column of `fields` |
| `concurrently` | migrations build the index with `CREATE INDEX CONCURRENTLY` |

`expr`, `where` and `opclass` are raw SQL over **column** names (the lower-cased
field name: `companyId` → `companyid`), and must not contain `;`.

`concurrently` changes nothing in the database `schema.sql` builds: an empty
database gains nothing from it, and `CREATE INDEX CONCURRENTLY` cannot run in a
transaction. schema.sql marks the statement with a trailing
`-- ang:concurrently`, and `ang migrate diff` writes that index as
`CREATE INDEX CONCURRENTLY IF NOT EXISTS` in a file headed
`-- atlas:txmode none`. A runner that wraps each file in a transaction
(`psql --single-transaction`) cannot apply such a file.

An entry that only has `fields` and `unique` renders exactly as before these
keys existed.

### Names longer than 63 bytes

Postgres keeps the first 63 bytes of a longer name and says so only in a NOTICE.
A generated name (`uidx_<table>_<all columns>`) can exceed that.

- Default (`target.sql_index_names` unset or `"truncate"`): the name is emitted
  as before, Postgres stores its first 63 bytes, and the build warns
  `SQL_INDEX_NAME_TRUNCATED` with the stored name. Pin it with
  `name: "<the stored name>"`: the database stays as it is and the warning goes.
  Existing databases already hold the truncated names, so the default does not
  rename anything.
- `target.sql_index_names: "hashed"`: a generated name over 63 bytes becomes its
  first 54 bytes + `_` + the first 8 hex digits of its SHA-256 — deterministic,
  and different for names that share a 63-byte prefix. For new projects, or
  after pinning the names a database already has.

Whatever the mode, two different indexes whose stored names are equal fail the
build with `SQL_INDEX_NAME_COLLISION` (before, `IF NOT EXISTS` skipped the
second one without a word), as does an index named like a table. The same
index declared twice (a field index and a one-field composite) stays harmless.

An explicit `name` longer than 63 bytes is an error.

## CHECK constraints

```cue
Counterparty: {
	@check(name="counterpartys_price_adjustment_bps_valid", expr="(priceadjustmentbps >= 0) AND (priceadjustmentbps <= 1000000)")
	@check(name="counterpartys_rounding_mode_valid", expr="roundingmode = ANY (ARRAY['none'::text, 'integer'::text, 'cent'::text])")
	...
}
```

Rendered after the table as
`DO $ang$ BEGIN ALTER TABLE … ADD CONSTRAINT … CHECK (…); EXCEPTION WHEN duplicate_object THEN NULL; END $ang$;`
so schema.sql stays re-runnable. Names are lower-case identifiers of at most 63
bytes, unique per table. (`checks:` is not a reserved entity key: entities
already use it as a field.)

## Raw DDL (triggers, functions)

```cue
AuditLog: {
	...
	ddl: """
		CREATE OR REPLACE FUNCTION auditlogs_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
		    RAISE EXCEPTION 'audit entries cannot be modified';
		END;
		$$;
		CREATE OR REPLACE TRIGGER auditlogs_append_only BEFORE DELETE OR UPDATE ON auditlogs
		    FOR EACH ROW EXECUTE FUNCTION auditlogs_append_only();
		"""
}
```

`ddl` is a string (or a list of strings) emitted verbatim after the entity's
table, indexes and checks, between `-- ang:ddl <Entity>` and `-- ang:ddl end`.
ANG does not parse it; keep it idempotent (`CREATE OR REPLACE`). System tables
(`outbox_events`, …) come first in schema.sql, so a trigger on them can live in
any entity's `ddl`. A field typed `ddl: string` stays a field: only a concrete
string is raw SQL.

## Migrations: `ang migrate diff`

```bash
ang migrate diff add_customer_tags [--dev-url URL] [--dir db/migrations] [--to db/schema/schema.sql] [--dry-run]
```

Atlas plans what the migration directory lacks compared with schema.sql
(`atlas schema diff`); ANG writes the file, keeping only additive, idempotent
statements:

- `CREATE TABLE IF NOT EXISTS`, `CREATE [UNIQUE] INDEX [CONCURRENTLY] IF NOT EXISTS`,
  `ALTER TABLE … ADD COLUMN IF NOT EXISTS`, `CREATE EXTENSION|SCHEMA|SEQUENCE IF NOT EXISTS`,
  `CREATE OR REPLACE FUNCTION|TRIGGER|VIEW`;
- `ADD CONSTRAINT` and `CREATE TYPE` wrapped in a `DO` block that ignores an
  existing object.

Drops, type changes, renames and `SET NOT NULL` are never written: each is
printed as `WARNING: skipped (not additive): …` (and listed as a comment at the
top of the file). A new `NOT NULL` column without `DEFAULT` gets a warning: it
fails on a table with rows. When nothing additive is left, no file is written.
The file is named `<UTC timestamp>_<name>.sql`, or one second after the newest
migration if that one is dated later; `atlas.sum` is rehashed when present.

Raw `ddl` blocks are left out of what Atlas sees (without `atlas login` Atlas
refuses a desired schema with functions, and it does not inspect triggers):
the command names the entities whose raw SQL needs a hand-written migration.

`scripts/atlas-diff.sh` calls `ang migrate diff` (`$ANG`, else `./bin/ang`,
else `ang`). `ang db status` lists what the database lacks; `ang db sync`
applies only those additive statements (never `atlas schema apply`, which
would drop every hand-made index).

### The dev database

`atlas.hcl` gets `dev = "docker://postgres/<target.postgres_version>/dev"`,
default **18**. `ang migrate diff` and `ang db` take the dev database from
`--dev-url`, then `ANG_ATLAS_DEV_URL`, then `atlas.hcl`, then
`docker://postgres/18/dev`. A `postgres://` dev URL must be an empty, throwaway
database, recreated for every run: Atlas cleans tables but not functions.

```cue
state: target: {
	postgres_version: "18"       // Atlas dev image
	sql_index_names:  "truncate" // or "hashed"
}
```

If the project's own `#Target` definition is closed, add both keys to it first.
