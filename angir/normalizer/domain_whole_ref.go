package normalizer

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/token"
)

// domainWholeReference names the domain definition an operation's whole
// input or output is written as — `output: domain.#QuoteView`, alone or
// unified with an inline struct (`domain.#QuoteView & {currency: string}`) —
// or "". The loader stubs the domain package while it evaluates cue/api, so
// the value itself carries none of the definition's fields: they come from
// the extracted entity (withDomainFields).
func domainWholeReference(v cue.Value) string {
	if name := domainReferenceInNode(v.Source()); name != "" {
		return name
	}
	if op, args := v.Expr(); op == cue.AndOp {
		for _, arg := range args {
			if name := domainWholeReference(arg); name != "" {
				return name
			}
		}
	}
	return ""
}

func domainReferenceInNode(node ast.Node) string {
	switch n := node.(type) {
	case *ast.Field:
		return domainReferenceInNode(n.Value)
	case *ast.ParenExpr:
		return domainReferenceInNode(n.X)
	case *ast.SelectorExpr:
		return domainSelectorName(n)
	case *ast.BinaryExpr:
		if n.Op == token.AND {
			if name := domainReferenceInNode(n.X); name != "" {
				return name
			}
			return domainReferenceInNode(n.Y)
		}
	}
	return ""
}

// withDomainFields puts the fields of the domain entity the value refers to
// in front of the fields parsed from the value itself (an inline part of the
// unification); a field written inline wins.
func withDomainFields(ent Entity, v cue.Value, entities []Entity, opName, side string) (Entity, error) {
	name := domainWholeReference(v)
	if name == "" {
		return ent, nil
	}
	var source *Entity
	for i := range entities {
		if entities[i].Name == name {
			source = &entities[i]
			break
		}
	}
	if source == nil {
		return ent, fmt.Errorf("%s of %s is domain.#%s, which cue/domain does not declare", side, opName, name)
	}
	inline := make(map[string]bool, len(ent.Fields))
	for _, f := range ent.Fields {
		inline[f.Name] = true
	}
	fields := make([]Field, 0, len(source.Fields)+len(ent.Fields))
	for _, f := range source.Fields {
		if !inline[f.Name] {
			fields = append(fields, f)
		}
	}
	ent.Fields = append(fields, ent.Fields...)
	return ent, nil
}
