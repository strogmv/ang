package schema

import "strings"

// #Index is one entry of an entity's indexes: [...] list; ANG emits it into
// db/schema/schema.sql. ANG checks the same keys when it reads a domain
// entity (an unknown key is an error), so `indexes: [...schema.#Index]` is
// optional. expr, where and opclass are raw SQL over column names (the
// lower-cased field names), not field names.
#Index: {
	// Entity field names; the index columns. Excludes expr.
	fields?: [...string] & [_, ...]
	// Raw key list of an expression index: "lower(email)" or
	// "companyid, lower(btrim(externalid))". Needs name.
	expr?: string & !=""
	// The index name. Default idx_<table>_<columns> (uidx_ when unique); a
	// generated name over 63 bytes is truncated by Postgres, see
	// target.sql_index_names.
	name?: =~"^[a-z_][a-z0-9_$]*$" & strings.MaxRunes(63)
	unique?: bool
	// Predicate of a partial index: "deletedat IS NULL".
	where?: string & !=""
	// Access method; btree when absent.
	using?: "btree" | "hash" | "gist" | "spgist" | "gin" | "brin"
	// Operator class for every column in fields: "gin_trgm_ops".
	opclass?: =~"^[a-z_][a-z0-9_.]*$"
	// Build the index with CREATE INDEX CONCURRENTLY in migrations written by
	// ang migrate diff. schema.sql (an empty database) ignores it.
	concurrently?: bool
}
