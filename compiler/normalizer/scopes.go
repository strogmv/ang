package normalizer

import (
	"strings"

	"cuelang.org/go/cue"
)

// ExtractScopes reads Scopes registry from cue/domain (optional).
// Expected shape:
//
//	Scopes: {
//	    TenderCreate: "tender:create"
//	    AITenderCreate: "tender:create" | "tender_template:create"
//	}
func (n *Normalizer) ExtractScopes(val cue.Value) ([]ScopeDef, error) {
	if !val.Exists() {
		return nil, nil
	}

	lookup := func(name string) cue.Value {
		v := val.LookupPath(cue.ParsePath(name))
		if v.Exists() {
			return v
		}
		return cue.Value{}
	}

	scopesVal := lookup("#Scopes")
	if !scopesVal.Exists() {
		scopesVal = lookup("Scopes")
	}
	if !scopesVal.Exists() {
		return nil, nil
	}

	var out []ScopeDef
	it, err := scopesVal.Fields()
	if err != nil {
		return nil, err
	}
	for it.Next() {
		name := it.Selector().String()
		vals := collectEnumStrings(it.Value())
		if len(vals) == 0 {
			continue
		}
		out = append(out, ScopeDef{
			Name:   name,
			Values: vals,
			Source: formatPos(it.Value()),
		})
	}
	return out, nil
}

// collectEnumStrings extracts string values from string, list, or disjunction.
func collectEnumStrings(v cue.Value) []string {
	k := v.IncompleteKind()
	switch k {
	case cue.StringKind:
		s, _ := v.String()
		if s != "" {
			return []string{strings.TrimSpace(s)}
		}
	case cue.ListKind:
		var vals []string
		it, _ := v.List()
		for it.Next() {
			s, _ := it.Value().String()
			s = strings.TrimSpace(s)
			if s != "" {
				vals = append(vals, s)
			}
		}
		return vals
	default:
		if op, args := v.Expr(); op == cue.OrOp {
			var vals []string
			for _, d := range args {
				if d.IncompleteKind() == cue.StringKind {
					s, _ := d.String()
					s = strings.TrimSpace(s)
					if s != "" {
						vals = append(vals, s)
					}
				}
			}
			return vals
		}
	}
	return nil
}
