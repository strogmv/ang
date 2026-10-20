// Package sqlmigrate turns a schema diff (the SQL Atlas plans to go from a
// database to db/schema/schema.sql) into a migration that only adds: every
// statement is additive and idempotent, and everything else — drops, type
// changes, renames, NOT NULL on an existing column — becomes a warning for a
// human to write by hand. A generated migration never deletes data, and it
// never drops an index or a constraint that exists only in the database.
package sqlmigrate

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultPostgresVersion is the major version of the Atlas dev database when
// target.postgres_version is not set.
const DefaultPostgresVersion = "18"

// DevURL is the Atlas dev database for a Postgres major version.
func DevURL(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = DefaultPostgresVersion
	}
	return "docker://postgres/" + version + "/dev"
}

// Plan is the additive part of a diff and what was left out.
type Plan struct {
	// Statements are additive and idempotent, each ending with ";".
	Statements []string
	// Warnings describe each statement of the diff that was not kept.
	Warnings []string
	// NoTransaction is set when a statement (CREATE INDEX CONCURRENTLY)
	// cannot run inside a transaction.
	NoTransaction bool
}

var (
	reCreateTable    = regexp.MustCompile(`(?is)^CREATE\s+(UNLOGGED\s+)?TABLE\s+(IF\s+NOT\s+EXISTS\s+)?(.*)$`)
	reCreateIndex    = regexp.MustCompile(`(?is)^CREATE\s+(UNIQUE\s+)?INDEX\s+(CONCURRENTLY\s+)?(IF\s+NOT\s+EXISTS\s+)?("[^"]+"|[A-Za-z_][A-Za-z0-9_$]*)\s+(ON\s.*)$`)
	reCreateIfNot    = regexp.MustCompile(`(?is)^CREATE\s+(EXTENSION|SCHEMA|SEQUENCE)\s+(IF\s+NOT\s+EXISTS\s+)?(.*)$`)
	reCreateReplace  = regexp.MustCompile(`(?is)^CREATE\s+(OR\s+REPLACE\s+)?(FUNCTION|PROCEDURE|TRIGGER|VIEW)\s+(.*)$`)
	reCreateType     = regexp.MustCompile(`(?is)^CREATE\s+TYPE\s`)
	reAlterTable     = regexp.MustCompile(`(?is)^ALTER\s+TABLE\s+(ONLY\s+)?("[^"]+"(?:\."[^"]+")?|[A-Za-z_][A-Za-z0-9_$.]*)\s+(.*)$`)
	reAddColumn      = regexp.MustCompile(`(?is)^ADD\s+COLUMN\s+(IF\s+NOT\s+EXISTS\s+)?(.*)$`)
	reAddConstraint  = regexp.MustCompile(`(?is)^ADD\s+CONSTRAINT\s`)
	reNotNull        = regexp.MustCompile(`(?i)\bNOT\s+NULL\b`)
	reDefault        = regexp.MustCompile(`(?i)\bDEFAULT\b`)
	reCommentOn      = regexp.MustCompile(`(?is)^COMMENT\s+ON\s`)
	reWhitespaceRuns = regexp.MustCompile(`\s+`)
)

// Safe keeps the additive statements of diffSQL. concurrent names the indexes
// (unquoted) to build with CREATE INDEX CONCURRENTLY.
func Safe(diffSQL string, concurrent map[string]bool) Plan {
	var p Plan
	if strings.HasPrefix(strings.TrimSpace(diffSQL), "Schemas are synced") {
		return p
	}
	skip := func(stmt, why string) {
		p.Warnings = append(p.Warnings, fmt.Sprintf("skipped (%s): %s", why, oneLine(stmt)))
	}
	// Constraints come after every column and index of the plan: Atlas puts
	// ADD CONSTRAINT … CHECK (col …) into the same ALTER TABLE as ADD COLUMN
	// col, and a CHECK written before its column fails.
	var constraints []string
	for _, stmt := range SplitStatements(diffSQL) {
		body := strings.TrimSuffix(strings.TrimSpace(stmt), ";")
		switch {
		case reCreateTable.MatchString(body):
			m := reCreateTable.FindStringSubmatch(body)
			p.Statements = append(p.Statements, "CREATE "+strings.ToUpper(strings.TrimSpace(m[1]))+spaceIf(m[1])+"TABLE IF NOT EXISTS "+m[3]+";")
		case reCreateIndex.MatchString(body):
			m := reCreateIndex.FindStringSubmatch(body)
			name := m[4]
			kind := "INDEX"
			if m[1] != "" {
				kind = "UNIQUE INDEX"
			}
			conc := ""
			if m[2] != "" || concurrent[strings.Trim(name, `"`)] {
				conc = "CONCURRENTLY "
				p.NoTransaction = true
			}
			p.Statements = append(p.Statements, "CREATE "+kind+" "+conc+"IF NOT EXISTS "+name+" "+m[5]+";")
		case reCreateIfNot.MatchString(body):
			m := reCreateIfNot.FindStringSubmatch(body)
			p.Statements = append(p.Statements, "CREATE "+strings.ToUpper(m[1])+" IF NOT EXISTS "+m[3]+";")
		case reCreateReplace.MatchString(body):
			m := reCreateReplace.FindStringSubmatch(body)
			p.Statements = append(p.Statements, "CREATE OR REPLACE "+strings.ToUpper(m[2])+" "+m[3]+";")
		case reCreateType.MatchString(body):
			p.Statements = append(p.Statements, ignoreDuplicate(body))
		case reCommentOn.MatchString(body):
			p.Statements = append(p.Statements, body+";")
		case reAlterTable.MatchString(body):
			m := reAlterTable.FindStringSubmatch(body)
			table := m[2]
			var columns []string
			for _, clause := range splitTopLevel(m[3], ',') {
				clause = strings.TrimSpace(clause)
				switch {
				case reAddConstraint.MatchString(clause):
					constraints = append(constraints, ignoreDuplicate("ALTER TABLE "+table+" "+clause))
				case reAddColumn.MatchString(clause):
					cm := reAddColumn.FindStringSubmatch(clause)
					if reNotNull.MatchString(cm[2]) && !reDefault.MatchString(cm[2]) {
						p.Warnings = append(p.Warnings, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s: NOT NULL without DEFAULT fails on a table that has rows", table, oneLine(cm[2])))
					}
					columns = append(columns, "ADD COLUMN IF NOT EXISTS "+cm[2])
				default:
					skip("ALTER TABLE "+table+" "+clause, "not additive")
				}
			}
			if len(columns) > 0 {
				p.Statements = append(p.Statements, "ALTER TABLE "+table+" "+strings.Join(columns, ", ")+";")
			}
		default:
			why := "not additive"
			if !strings.HasPrefix(strings.ToUpper(body), "DROP") {
				why = "not recognised as additive"
			}
			skip(body+";", why)
		}
	}
	p.Statements = append(p.Statements, constraints...)
	return p
}

// ignoreDuplicate wraps a statement that has no IF NOT EXISTS form so that it
// is a no-op when the object exists.
func ignoreDuplicate(stmt string) string {
	return "DO $ang$ BEGIN " + strings.TrimSuffix(strings.TrimSpace(stmt), ";") +
		"; EXCEPTION WHEN duplicate_object OR duplicate_table THEN NULL; END $ang$;"
}

// Render is the migration file for a plan.
func Render(p Plan) string {
	var b strings.Builder
	if p.NoTransaction {
		b.WriteString("-- atlas:txmode none\n\n")
	}
	b.WriteString("-- Generated by ang migrate diff: additive, idempotent statements only.\n")
	if p.NoTransaction {
		b.WriteString("-- CREATE INDEX CONCURRENTLY cannot run inside a transaction: apply this file without one.\n")
	}
	if len(p.Warnings) > 0 {
		b.WriteString("-- Left out, write by hand if intended:\n")
		for _, w := range p.Warnings {
			b.WriteString("--   " + w + "\n")
		}
	}
	b.WriteString("\n")
	for _, s := range p.Statements {
		b.WriteString(s + "\n")
	}
	return b.String()
}

// SplitStatements splits SQL into statements at top-level semicolons, keeping
// quoted strings, quoted identifiers and dollar-quoted bodies whole and
// dropping comments. Statements keep their trailing ";".
func SplitStatements(sql string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" && s != ";" {
			out = append(out, s)
		}
		cur.Reset()
	}
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		switch {
		case c == '-' && i+1 < len(sql) && sql[i+1] == '-':
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			cur.WriteByte('\n')
		case c == '/' && i+1 < len(sql) && sql[i+1] == '*':
			end := strings.Index(sql[i+2:], "*/")
			if end < 0 {
				i = len(sql)
			} else {
				i += end + 3
			}
			cur.WriteByte(' ')
		case c == '\'' || c == '"':
			j := i + 1
			for j < len(sql) {
				if sql[j] == c {
					if j+1 < len(sql) && sql[j+1] == c {
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j >= len(sql) {
				j = len(sql) - 1
			}
			cur.WriteString(sql[i : j+1])
			i = j
		case c == '$':
			if tag := dollarTag(sql[i:]); tag != "" {
				end := strings.Index(sql[i+len(tag):], tag)
				if end < 0 {
					cur.WriteString(sql[i:])
					i = len(sql)
					break
				}
				stop := i + len(tag) + end + len(tag)
				cur.WriteString(sql[i:stop])
				i = stop - 1
				break
			}
			cur.WriteByte(c)
		case c == ';':
			cur.WriteByte(';')
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
}

var reDollarTag = regexp.MustCompile(`^\$[A-Za-z_]*\$`)

func dollarTag(s string) string {
	return reDollarTag.FindString(s)
}

// splitTopLevel splits s at sep outside parentheses and quotes.
func splitTopLevel(s string, sep byte) []string {
	var out []string
	depth := 0
	start := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == sep && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func oneLine(s string) string {
	return strings.TrimSpace(reWhitespaceRuns.ReplaceAllString(s, " "))
}

func spaceIf(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return " "
}

// ConcurrentlyMarker follows an index statement in db/schema/schema.sql that
// migrations build with CREATE INDEX CONCURRENTLY (index key
// concurrently: true). schema.sql builds an empty database, where
// CONCURRENTLY buys nothing and cannot run inside a transaction.
const ConcurrentlyMarker = "-- ang:concurrently"

// ConcurrentIndexNames returns the names of the indexes schema.sql marks with
// ConcurrentlyMarker.
func ConcurrentIndexNames(schemaSQL string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(schemaSQL, "\n") {
		if !strings.HasSuffix(strings.TrimSpace(line), ConcurrentlyMarker) {
			continue
		}
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "EXISTS" && i+1 < len(fields) {
				out[strings.Trim(fields[i+1], `"`)] = true
				break
			}
		}
	}
	return out
}

var rePostgresVersion = regexp.MustCompile(`^[0-9]{2}$`)

// ValidPostgresVersion reports whether v is "" or a Postgres major version.
func ValidPostgresVersion(v string) bool {
	return v == "" || rePostgresVersion.MatchString(v)
}

// DDLBeginMarker and DDLEndMarker enclose an entity's raw SQL (entity key
// ddl) in schema.sql.
const (
	DDLBeginMarker = "-- ang:ddl"
	DDLEndMarker   = "-- ang:ddl end"
)

// StripRawDDL removes the raw SQL blocks from schema.sql and names the
// entities they belong to. Atlas without a login refuses a desired schema
// with functions, and it would not see triggers anyway: a migration for raw
// SQL is written by hand.
func StripRawDDL(schemaSQL string) (string, []string) {
	var out strings.Builder
	var entities []string
	inside := false
	for _, line := range strings.SplitAfter(schemaSQL, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inside && strings.HasPrefix(trimmed, DDLBeginMarker+" ") && trimmed != DDLEndMarker:
			inside = true
			entities = append(entities, strings.TrimSpace(strings.TrimPrefix(trimmed, DDLBeginMarker)))
		case inside && trimmed == DDLEndMarker:
			inside = false
		case !inside:
			out.WriteString(line)
		}
	}
	return out.String(), entities
}
