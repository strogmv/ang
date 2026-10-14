package emitter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/sqlmigrate"
)

// postgresMaxIdentifier is NAMEDATALEN-1: Postgres keeps the first 63 bytes of
// a longer name and says so only in a NOTICE.
const postgresMaxIdentifier = 63

// SQL index naming modes (target.sql_index_names).
const (
	// SQLIndexNamesTruncate keeps a generated name as it is even when it is
	// longer than 63 bytes; Postgres stores its first 63 bytes. The default:
	// existing databases already hold those truncated names.
	SQLIndexNamesTruncate = "truncate"
	// SQLIndexNamesHashed shortens a generated name longer than 63 bytes to
	// its first 54 bytes, "_" and 8 hex digits of its SHA-256.
	SQLIndexNamesHashed = "hashed"
)

// CodeSQLIndexNameTruncated is the warning for a generated index name that
// Postgres truncates.
const CodeSQLIndexNameTruncated = "SQL_INDEX_NAME_TRUNCATED"

var (
	sqlIdentifierRe = regexp.MustCompile(`^[a-z_][a-z0-9_$]*$`)
	sqlOpclassRe    = regexp.MustCompile(`^[a-z_][a-z0-9_.]*$`)
	sqlIndexMethods = map[string]bool{"btree": true, "hash": true, "gist": true, "spgist": true, "gin": true, "brin": true}
)

// ValidSQLIndexNames reports whether mode is a known target.sql_index_names value.
func ValidSQLIndexNames(mode string) bool {
	switch mode {
	case "", SQLIndexNamesTruncate, SQLIndexNamesHashed:
		return true
	}
	return false
}

// hashedSQLName shortens name to 63 bytes: its first 54 bytes, "_" and the
// first 8 hex digits of the SHA-256 of the whole name.
func hashedSQLName(name string) string {
	if len(name) <= postgresMaxIdentifier {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	return name[:postgresMaxIdentifier-9] + "_" + hex.EncodeToString(sum[:])[:8]
}

// effectiveSQLName is the name Postgres stores.
func effectiveSQLName(name string) string {
	if len(name) <= postgresMaxIdentifier {
		return name
	}
	return name[:postgresMaxIdentifier]
}

// sqlIndexName returns the name to emit for a generated index name.
func (e *Emitter) sqlIndexName(generated, source string) string {
	if len(generated) <= postgresMaxIdentifier {
		return generated
	}
	if e.SQLIndexNames == SQLIndexNamesHashed {
		return hashedSQLName(generated)
	}
	if e.WarningSink != nil {
		e.WarningSink(normalizer.Warning{
			Kind:     "sql",
			Code:     CodeSQLIndexNameTruncated,
			Severity: "warn",
			Message: fmt.Sprintf("index name %s is %d bytes; Postgres keeps the first 63: %s. Pin that name with name: %q in the index, or set target.sql_index_names: %q for new projects",
				generated, len(generated), effectiveSQLName(generated), effectiveSQLName(generated), SQLIndexNamesHashed),
			Path: source,
		})
	}
	return generated
}

// sqlRelationNames tracks the names schema.sql gives to tables and indexes:
// Postgres keeps them in one namespace per schema, and an index whose
// (truncated) name is taken is skipped without an error by IF NOT EXISTS.
type sqlRelationNames struct {
	tables  map[string]string // name -> source
	indexes map[string]sqlIndexSeen
}

type sqlIndexSeen struct {
	name, table, stmt, source string
}

func newSQLRelationNames() *sqlRelationNames {
	return &sqlRelationNames{tables: map[string]string{}, indexes: map[string]sqlIndexSeen{}}
}

func (n *sqlRelationNames) addTable(name, source string) {
	n.tables[effectiveSQLName(name)] = source
}

// addIndex records an index. The same statement twice is harmless (the second
// is a no-op); a different index under the same stored name is an error.
func (n *sqlRelationNames) addIndex(name, table, stmt, source string) error {
	eff := effectiveSQLName(name)
	if src, ok := n.tables[eff]; ok {
		return fmt.Errorf("SQL_INDEX_NAME_COLLISION: index %s on %s (%s) has the name of a table (%s)", name, table, source, src)
	}
	if prev, ok := n.indexes[eff]; ok {
		if prev.stmt == stmt {
			return nil
		}
		return fmt.Errorf("SQL_INDEX_NAME_COLLISION: two different indexes are stored as %s:\n  %s (%s)\n  %s (%s)\ngive one of them name: \"…\"",
			eff, prev.stmt, prev.source, stmt, source)
	}
	n.indexes[eff] = sqlIndexSeen{name: name, table: table, stmt: stmt, source: source}
	return nil
}

// sqlIndexStatement renders one entry of an entity's indexes: [...] list. An
// entry that only has fields and unique renders exactly as before the other
// keys existed.
func (e *Emitter) sqlIndexStatement(entity normalizer.Entity, tableName string, index normalizer.IndexDef, names *sqlRelationNames) (string, error) {
	where := index.Source
	if where == "" {
		where = entity.Source
	}
	fail := func(format string, args ...any) (string, error) {
		return "", fmt.Errorf("%s: entity %s: "+format, append([]any{where, entity.Name}, args...)...)
	}
	if len(index.Fields) == 0 && index.Expr == "" {
		return fail("an index needs fields or expr")
	}
	if len(index.Fields) > 0 && index.Expr != "" {
		return fail("an index has either fields or expr, not both (put the columns into expr)")
	}
	if index.Expr != "" && index.Name == "" {
		return fail("an expression index needs name: \"…\"")
	}
	if index.Expr != "" && index.Opclass != "" {
		return fail("opclass applies to fields; write it inside expr")
	}
	for key, raw := range map[string]string{"expr": index.Expr, "where": index.Where} {
		if strings.Contains(raw, ";") {
			return fail("index %s must not contain ';': %s", key, raw)
		}
	}
	method := strings.ToLower(index.Using)
	if method != "" && !sqlIndexMethods[method] {
		return fail("unknown index method using: %q (btree, hash, gist, spgist, gin, brin)", index.Using)
	}
	if index.Opclass != "" && !sqlOpclassRe.MatchString(index.Opclass) {
		return fail("opclass %q is not an operator class name", index.Opclass)
	}

	columns := make([]string, 0, len(index.Fields))
	for _, fieldName := range index.Fields {
		found := false
		for _, field := range entity.Fields {
			if strings.EqualFold(field.Name, fieldName) {
				columns = append(columns, DBName(field.Name))
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("entity %s index references unknown field %s", entity.Name, fieldName)
		}
	}

	kind := "INDEX"
	prefix := "idx"
	if index.Unique {
		kind = "UNIQUE INDEX"
		prefix = "uidx"
	}
	var idxName string
	if index.Name != "" {
		if !sqlIdentifierRe.MatchString(index.Name) {
			return fail("index name %q must be a lower-case SQL identifier", index.Name)
		}
		if len(index.Name) > postgresMaxIdentifier {
			return fail("index name %s is %d bytes; Postgres allows 63", index.Name, len(index.Name))
		}
		idxName = index.Name
	} else {
		idxName = e.sqlIndexName(fmt.Sprintf("%s_%s_%s", prefix, tableName, strings.Join(columns, "_")), where)
	}

	var stmt string
	plain := index.Expr == "" && index.Where == "" && method == "" && index.Opclass == ""
	if plain {
		stmt = fmt.Sprintf("CREATE %s IF NOT EXISTS %s ON %s (%s);", kind, idxName, tableName, strings.Join(columns, ", "))
	} else {
		keys := index.Expr
		if keys == "" {
			parts := make([]string, len(columns))
			for i, c := range columns {
				parts[i] = c
				if index.Opclass != "" {
					parts[i] += " " + index.Opclass
				}
			}
			keys = strings.Join(parts, ", ")
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CREATE %s IF NOT EXISTS %s ON %s", kind, idxName, tableName)
		if method != "" {
			b.WriteString(" USING " + method)
		}
		fmt.Fprintf(&b, " (%s)", keys)
		if index.Where != "" {
			fmt.Fprintf(&b, " WHERE %s", index.Where)
		}
		b.WriteString(";")
		stmt = b.String()
	}
	if err := names.addIndex(idxName, tableName, stmt, where); err != nil {
		return "", err
	}
	if index.Concurrently {
		// schema.sql builds an empty database, where CONCURRENTLY buys nothing
		// and cannot run inside a transaction; the marker tells
		// ang migrate diff to build this index concurrently.
		stmt += " " + sqlmigrate.ConcurrentlyMarker
	}
	return stmt, nil
}

// sqlCheckStatement renders a named CHECK constraint so that schema.sql stays
// re-runnable: adding a constraint that exists raises duplicate_object.
func sqlCheckStatement(entity normalizer.Entity, tableName string, check normalizer.CheckDef, seen map[string]bool) (string, error) {
	where := check.Source
	if where == "" {
		where = entity.Source
	}
	if !sqlIdentifierRe.MatchString(check.Name) || len(check.Name) > postgresMaxIdentifier {
		return "", fmt.Errorf("%s: entity %s: check name %q must be a lower-case SQL identifier of at most 63 bytes", where, entity.Name, check.Name)
	}
	if strings.Contains(check.Expr, ";") || strings.Contains(check.Expr, "$ang$") {
		return "", fmt.Errorf("%s: entity %s: check %s: expr must not contain ';' or $ang$", where, entity.Name, check.Name)
	}
	if seen[check.Name] {
		return "", fmt.Errorf("%s: entity %s: two checks are named %s", where, entity.Name, check.Name)
	}
	seen[check.Name] = true
	return fmt.Sprintf("DO $ang$ BEGIN ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s); EXCEPTION WHEN duplicate_object THEN NULL; END $ang$;",
		tableName, check.Name, check.Expr), nil
}
