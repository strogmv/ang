package normalizer

import (
	"fmt"
	"go/token"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// finderParamTypes maps the types a sql: finder's params: may name to Go.
// uuid and []uuid are strings in Go; Postgres reads them as text and casts
// ($1::uuid, $2::uuid[]) or compares them with a uuid column directly.
var finderParamTypes = map[string]string{
	"string":    "string",
	"text":      "string",
	"uuid":      "string",
	"int":       "int",
	"int64":     "int64",
	"float":     "float64",
	"float64":   "float64",
	"bool":      "bool",
	"time":      "time.Time",
	"time.Time": "time.Time",
	"[]string":  "[]string",
	"[]uuid":    "[]string",
	"[]int":     "[]int",
	"[]int64":   "[]int64",
}

// FinderParamGoType returns the Go type of a params: type, ok false for an
// unknown one.
func FinderParamGoType(spec string) (string, bool) {
	t, ok := finderParamTypes[strings.TrimSpace(spec)]
	return t, ok
}

// FinderParamTypeNames lists the types params: accepts, sorted.
func FinderParamTypeNames() []string {
	out := make([]string, 0, len(finderParamTypes))
	for k := range finderParamTypes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type finderExtensions struct {
	Params     []FinderParam
	Paged      *bool
	Kind       string
	OnConflict []string
	SelectAuto bool
}

// extractFinderExtensions reads the finder keys added after the first
// version of cue/repo — params, paged, kind, on_conflict, select: "auto" —
// and rejects combinations that would generate a method that cannot work.
func extractFinderExtensions(fv cue.Value, hasSQL, hasWhere bool, selectFields []string) (finderExtensions, error) {
	var ext finderExtensions

	if v := fv.LookupPath(cue.ParsePath("params")); v.Exists() {
		list, err := v.List()
		if err != nil {
			return ext, fmt.Errorf("params: must be a list of {name, type}: %w", err)
		}
		seen := map[string]bool{}
		for idx := 0; list.Next(); idx++ {
			pv := list.Value()
			pname, err := finderString(pv, "name")
			if err != nil {
				return ext, fmt.Errorf("params[%d].name: %w", idx, err)
			}
			spec, err := finderString(pv, "type")
			if err != nil {
				return ext, fmt.Errorf("params[%d].type: %w", idx, err)
			}
			pname = strings.TrimSpace(pname)
			if !token.IsIdentifier(pname) || token.IsKeyword(pname) {
				return ext, fmt.Errorf("params[%d]: name %q is not a Go identifier", idx, pname)
			}
			if pname == "ctx" || pname == "r" {
				return ext, fmt.Errorf("params[%d]: name %q is taken by the generated method", idx, pname)
			}
			if seen[pname] {
				return ext, fmt.Errorf("params[%d]: name %q is declared twice", idx, pname)
			}
			seen[pname] = true
			goType, ok := FinderParamGoType(spec)
			if !ok {
				return ext, fmt.Errorf("params[%d] %q: unknown type %q; use one of %s", idx, pname, spec, strings.Join(FinderParamTypeNames(), ", "))
			}
			ext.Params = append(ext.Params, FinderParam{Name: pname, Type: goType, Spec: strings.TrimSpace(spec)})
		}
		if len(ext.Params) > 0 {
			if !hasSQL {
				return ext, fmt.Errorf("params: is for sql: finders; a finder without sql: takes its parameters from where:")
			}
			if hasWhere {
				return ext, fmt.Errorf("params: and where: both declare parameters; keep params: and drop where: (a sql: finder does not use where: for its SQL)")
			}
		}
	}

	if v := fv.LookupPath(cue.ParsePath("paged")); v.Exists() {
		b, err := v.Bool()
		if err != nil {
			return ext, fmt.Errorf("paged: must be true or false: %w", err)
		}
		ext.Paged = &b
		if b {
			for _, p := range ext.Params {
				if p.Name == "limit" || p.Name == "offset" {
					return ext, fmt.Errorf("paged: true adds the parameters limit and offset; params: must not declare %q", p.Name)
				}
			}
		}
	}

	kind, err := finderString(fv, "kind")
	if err != nil {
		return ext, fmt.Errorf("kind: %w", err)
	}
	ext.Kind = strings.TrimSpace(kind)
	for _, key := range []string{"on_conflict", "onConflict"} {
		v := fv.LookupPath(cue.ParsePath(key))
		if !v.Exists() {
			continue
		}
		list, err := v.List()
		if err != nil {
			return ext, fmt.Errorf("%s: must be a list of entity fields: %w", key, err)
		}
		for list.Next() {
			s, err := list.Value().String()
			if err != nil {
				return ext, fmt.Errorf("%s: %w", key, err)
			}
			if s = strings.TrimSpace(s); s != "" {
				ext.OnConflict = append(ext.OnConflict, s)
			}
		}
	}
	switch ext.Kind {
	case "":
		if len(ext.OnConflict) > 0 {
			return ext, fmt.Errorf(`on_conflict: needs kind: "upsert"`)
		}
	case "upsert":
		if len(ext.OnConflict) == 0 {
			return ext, fmt.Errorf(`kind: "upsert" needs on_conflict: [fields] — the columns of a unique index`)
		}
		if hasSQL || hasWhere || len(ext.Params) > 0 || ext.Paged != nil {
			return ext, fmt.Errorf(`kind: "upsert" writes its own INSERT … ON CONFLICT; it takes no sql:, where:, params: or paged:`)
		}
	default:
		return ext, fmt.Errorf(`kind: %q is unknown; the only kind is "upsert"`, ext.Kind)
	}

	if len(selectFields) == 1 && strings.TrimSpace(selectFields[0]) == "auto" {
		if !hasSQL {
			return ext, fmt.Errorf(`select: "auto" is for sql: finders; a finder without sql: always lists its columns`)
		}
		ext.SelectAuto = true
	}
	return ext, nil
}
