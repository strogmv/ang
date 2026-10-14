package normalizer

import (
	"fmt"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// indexKeys are the keys an entry of an entity's indexes: [...] list may have.
var indexKeys = map[string]bool{
	"fields": true, "unique": true, "name": true, "expr": true, "where": true,
	"using": true, "opclass": true, "concurrently": true,
}

// parseEntityIndexes reads the entity's indexes: [...] list. An unknown key
// (a typo such as wher:) or a value of the wrong type is an error with the
// position of the entry: an index that silently lost its WHERE would be a
// different index.
func parseEntityIndexes(entityVal cue.Value) ([]IndexDef, error) {
	indexVal := entityVal.LookupPath(cue.ParsePath("indexes"))
	if !indexVal.Exists() {
		return nil, nil
	}
	iter, err := indexVal.List()
	if err != nil {
		return nil, fmt.Errorf("%s: indexes must be a list: %w", formatPos(indexVal), err)
	}
	var out []IndexDef
	for iter.Next() {
		iv := iter.Value()
		pos := formatPos(iv)
		fieldIter, err := iv.Fields()
		if err != nil {
			return nil, fmt.Errorf("%s: an index must be a struct: %w", pos, err)
		}
		for fieldIter.Next() {
			key := strings.TrimSuffix(fieldIter.Selector().String(), "?")
			if !indexKeys[key] {
				return nil, fmt.Errorf("%s: unknown index key %q (known: %s)", pos, key, strings.Join(sortedKeys(indexKeys), ", "))
			}
		}
		idx := IndexDef{Source: pos}
		if fv := iv.LookupPath(cue.ParsePath("fields")); fv.Exists() {
			fit, err := fv.List()
			if err != nil {
				return nil, fmt.Errorf("%s: index fields must be a list of field names: %w", pos, err)
			}
			for fit.Next() {
				s, err := fit.Value().String()
				if err != nil {
					return nil, fmt.Errorf("%s: index fields must be a list of field names: %w", pos, err)
				}
				if s = strings.TrimSpace(s); s != "" {
					idx.Fields = append(idx.Fields, s)
				}
			}
		}
		for key, dst := range map[string]*string{
			"name": &idx.Name, "expr": &idx.Expr, "where": &idx.Where, "using": &idx.Using, "opclass": &idx.Opclass,
		} {
			v := iv.LookupPath(cue.ParsePath(key))
			if !v.Exists() {
				continue
			}
			s, err := v.String()
			if err != nil {
				return nil, fmt.Errorf("%s: index %s must be a string: %w", pos, key, err)
			}
			*dst = strings.TrimSpace(s)
		}
		for key, dst := range map[string]*bool{"unique": &idx.Unique, "concurrently": &idx.Concurrently} {
			v := iv.LookupPath(cue.ParsePath(key))
			if !v.Exists() {
				continue
			}
			b, err := v.Bool()
			if err != nil {
				return nil, fmt.Errorf("%s: index %s must be a bool: %w", pos, key, err)
			}
			*dst = b
		}
		// An entry with neither fields nor expr was skipped before; keep that.
		if len(idx.Fields) == 0 && idx.Expr == "" {
			continue
		}
		out = append(out, idx)
	}
	return out, nil
}

// parseEntityChecks reads @check(name="…", expr="…") attributes of the entity.
// An entity may carry several.
func parseEntityChecks(val cue.Value) ([]CheckDef, error) {
	var out []CheckDef
	pos := formatPos(val)
	for _, attr := range val.Attributes(cue.ValueAttr | cue.FieldAttr | cue.DeclAttr) {
		if attr.Name() != "check" {
			continue
		}
		name, okName, _ := attr.Lookup(0, "name")
		expr, okExpr, _ := attr.Lookup(0, "expr")
		name = strings.TrimSpace(strings.Trim(name, `"`))
		expr = strings.TrimSpace(expr)
		if !okName || !okExpr || name == "" || expr == "" {
			return nil, fmt.Errorf("%s: @check needs name=\"…\" and expr=\"…\": @check(%s)", pos, attr.Contents())
		}
		out = append(out, CheckDef{Name: name, Expr: expr, Source: pos})
	}
	return out, nil
}

// isEntityDDLValue says whether the entity key ddl holds raw SQL (a concrete
// string or list of strings) rather than being a field named ddl.
func isEntityDDLValue(v cue.Value) bool {
	switch v.Kind() {
	case cue.StringKind:
		return true
	case cue.ListKind:
		iter, err := v.List()
		if err != nil {
			return false
		}
		for iter.Next() {
			if iter.Value().Kind() != cue.StringKind {
				return false
			}
		}
		return true
	}
	return false
}

// parseEntityDDL reads the entity key ddl: """…""" (or a list of such strings).
func parseEntityDDL(entityVal cue.Value) ([]string, error) {
	v := entityVal.LookupPath(cue.ParsePath("ddl"))
	if !v.Exists() || !isEntityDDLValue(v) {
		return nil, nil
	}
	var out []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if v.Kind() == cue.StringKind {
		s, _ := v.String()
		add(s)
		return out, nil
	}
	iter, _ := v.List()
	for iter.Next() {
		s, _ := iter.Value().String()
		add(s)
	}
	return out, nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
