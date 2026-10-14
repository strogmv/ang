package normalizer

import (
	"fmt"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
)

// Table naming modes (target.table_names).
const (
	// TableNamesLegacy names a table lower(Entity) + "s": Company → companys,
	// CompanyAddress → companyaddresss. The default, so a project's existing
	// database keeps its tables.
	TableNamesLegacy = "legacy"
	// TableNamesPlural names a table lower(Entity) in its English plural:
	// Company → companies, CompanyAddress → companyaddresses.
	TableNamesPlural = "plural"
)

// ValidTableNames reports whether mode is a known target.table_names value.
func ValidTableNames(mode string) bool {
	switch mode {
	case "", TableNamesLegacy, TableNamesPlural:
		return true
	}
	return false
}

// TableName is the SQL table (and MongoDB collection) of an entity: the
// override when the entity declares @db_table(name="…"), otherwise its
// lower-cased name made plural by mode (target.table_names). Every emitter
// that names a table goes through Entity.TableName, which holds this value.
func TableName(entity, override, mode string) string {
	if override != "" {
		return override
	}
	base := strings.ToLower(entity)
	if mode == TableNamesPlural {
		return pluralize(base)
	}
	return base + "s"
}

// pluralize makes a lower-case word plural by the regular English rules:
//
//	consonant + y → ies     (company → companies, but key → keys)
//	s, x, z, ch, sh → + es  (address → addresses, status → statuses, tax → taxes)
//	consonant + s → as is   (terms, settings: already plural)
//	anything else → + s
//
// Irregular plurals (person, child, …) need @db_table(name="…").
func pluralize(w string) string {
	n := len(w)
	if n == 0 {
		return w
	}
	last := w[n-1]
	prev := byte(0)
	if n > 1 {
		prev = w[n-2]
	}
	switch {
	case last == 'y' && n > 1 && !isVowel(prev):
		return w[:n-1] + "ies"
	case last == 's' && n > 1 && prev != 's' && !isVowel(prev):
		return w
	case last == 's', last == 'x', last == 'z', strings.HasSuffix(w, "ch"), strings.HasSuffix(w, "sh"):
		return w + "es"
	}
	return w + "s"
}

func isVowel(c byte) bool {
	switch c {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// TableName is the entity's SQL table (MongoDB collection): Table as resolved
// by the normalizer, or the legacy name for an entity built elsewhere.
func (e Entity) TableName() string {
	if e.Table != "" {
		return e.Table
	}
	return TableName(e.Name, "", "")
}

var sqlIdentifierRE = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// parseEntityDBTable reads @db_table(name="…") on an entity: the table name
// that replaces the generated one. (@table is the SDK table attribute of list
// operations, so the entity attribute has its own name.)
func parseEntityDBTable(val cue.Value) (string, error) {
	pos := formatPos(val)
	name := ""
	for _, attr := range val.Attributes(cue.ValueAttr | cue.FieldAttr | cue.DeclAttr) {
		if attr.Name() != "db_table" {
			continue
		}
		v, ok, _ := attr.Lookup(0, "name")
		if !ok {
			return "", fmt.Errorf("%s: @db_table needs name=\"…\": @db_table(%s)", pos, attr.Contents())
		}
		v = strings.TrimSpace(strings.Trim(v, `"`))
		if !sqlIdentifierRE.MatchString(v) || len(v) > 63 {
			return "", fmt.Errorf("%s: @db_table name %q must be a lower-case SQL identifier of at most 63 bytes", pos, v)
		}
		if name != "" && name != v {
			return "", fmt.Errorf("%s: two @db_table names: %q and %q", pos, name, v)
		}
		name = v
	}
	return name, nil
}

// ProjectTableNames is the table naming mode of a project: target.table_names,
// which every target must agree on (they share one database).
func ProjectTableNames(targets []TargetDef) (string, error) {
	mode, from := "", ""
	for _, t := range targets {
		if !ValidTableNames(t.TableNames) {
			return "", fmt.Errorf("target %s: table_names must be %q or %q, got %q", t.Name, TableNamesLegacy, TableNamesPlural, t.TableNames)
		}
		m := t.TableNames
		if m == "" {
			m = TableNamesLegacy
		}
		if from != "" && m != mode {
			return "", fmt.Errorf("targets %s and %s disagree on table_names (%q, %q): they share one database", from, t.Name, mode, m)
		}
		mode, from = m, t.Name
	}
	if mode == TableNamesLegacy {
		mode = ""
	}
	return mode, nil
}
