package compiler

import (
	"go/scanner"
	"go/token"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/pkg/names"
)

// typeObjectsTheCodeReachesInto gives an inline object of an operation's
// input or output its named Go struct (<Op>Response<Field>, the name its
// TypeScript, Zod and OpenAPI shape already has) when the operation's own Go
// reaches into it. Such code cannot compile against the loose `any` an object
// gets otherwise. The code reaches into an object when
//
//   - a selector chain from req, resp or out selects one of its fields:
//     resp.Interest.Signs30d, resp.Pages[i].Settings.Remind;
//   - the same from a variable holding a list item: page := port.XPagesItem{}
//     or for _, page := range resp.Pages, then page.Settings.Remind;
//   - it names the object's type: port.GetXResponseInterest{...}.
//
// Every other inline object stays loose: flows fill those from map literals,
// domain JSON and structurally equal domain structs (typing all of them broke
// ~65 flows of dealingi-back, 3fbd839). A promoted object's own objects follow
// the same rule, and an object reached into makes every object around it a
// struct.
func typeObjectsTheCodeReachesInto(services []normalizer.Service) {
	for si := range services {
		for mi := range services[si].Methods {
			method := &services[si].Methods[mi]
			code := normalizer.MethodCodeText(*method)
			if strings.TrimSpace(code) == "" {
				continue
			}
			reached := objectsReachedByCode(method, code)
			if len(reached) == 0 {
				continue
			}
			promoteReachedObjects(method.Input.Fields, reached)
			promoteReachedObjects(method.Output.Fields, reached)
		}
	}
}

// promoteReachedObjects turns the reached loose objects among fields (and
// those around a reached one) into their named struct; it reports whether it
// promoted any.
func promoteReachedObjects(fields []normalizer.Field, reached map[string]bool) bool {
	promoted := false
	for i := range fields {
		f := &fields[i]
		if len(f.ItemFields) == 0 || f.ItemTypeName == "" {
			continue
		}
		switch {
		case isLooseObject(*f):
			inner := promoteReachedObjects(f.ItemFields, reached)
			if inner || reached[f.ItemTypeName] {
				f.Type = f.ItemTypeName
				delete(f.Metadata, "shape")
				promoted = true
			}
		case isObjectList(*f):
			if promoteReachedObjects(f.ItemFields, reached) {
				promoted = true
			}
		}
	}
	return promoted
}

// isLooseObject reports an inline object kept as any in Go (not a map, whose
// shape is map[string]T).
func isLooseObject(f normalizer.Field) bool {
	shape, _ := f.Metadata["shape"].(string)
	return shape != "" && shape == f.ItemTypeName && len(f.ItemFields) > 0
}

// isObjectList reports a list of inline objects ([]<Op>Response<Field>Item).
func isObjectList(f normalizer.Field) bool {
	return f.ItemTypeName != "" && len(f.ItemFields) > 0 && f.Type == "[]"+f.ItemTypeName
}

type codeToken struct {
	tok token.Token
	lit string
}

// objectsReachedByCode answers the type names of the loose objects of the
// method's input and output that code reaches into.
func objectsReachedByCode(method *normalizer.Method, code string) map[string]bool {
	// Every named nested type: the loose objects and the list items.
	byType := map[string][]normalizer.Field{}
	loose := map[string]bool{}
	var index func([]normalizer.Field)
	index = func(fields []normalizer.Field) {
		for _, f := range fields {
			switch {
			case isLooseObject(f):
				loose[f.ItemTypeName] = true
			case isObjectList(f):
			default:
				continue
			}
			byType[f.ItemTypeName] = f.ItemFields
			index(f.ItemFields)
		}
	}
	index(method.Input.Fields)
	index(method.Output.Fields)
	if len(loose) == 0 {
		return nil
	}

	toks := scanCodeTokens(code)
	reached := map[string]bool{}
	roots := map[string][]normalizer.Field{
		"req":  method.Input.Fields,
		"resp": method.Output.Fields,
		"out":  method.Output.Fields,
	}
	// Variables holding a list item, and objects named by their type.
	for i, t := range toks {
		if t.tok != token.IDENT {
			continue
		}
		if loose[t.lit] {
			reached[t.lit] = true
		}
		if typeName := declaredTypeAt(toks, i); typeName != "" && byType[typeName] != nil && !loose[typeName] {
			roots[t.lit] = byType[typeName]
		}
		if item := rangeItemAt(toks, i, roots); item != nil {
			roots[t.lit] = item
		}
	}
	for i, t := range toks {
		if t.tok != token.IDENT || roots[t.lit] == nil || (i > 0 && toks[i-1].tok == token.PERIOD) {
			continue
		}
		walkSelectorChain(roots[t.lit], selectorChain(toks, i+1), reached)
	}
	return reached
}

func scanCodeTokens(code string) []codeToken {
	src := []byte(code)
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, func(token.Position, string) {}, 0)
	var out []codeToken
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.SEMICOLON && lit == "\n" {
			continue
		}
		out = append(out, codeToken{tok: tok, lit: lit})
	}
}

// declaredTypeAt answers T for `x := T{`, `x := &T{`, `var x T` (T possibly
// port.T) with x at toks[i].
func declaredTypeAt(toks []codeToken, i int) string {
	j := i + 1
	switch {
	case j < len(toks) && toks[j].tok == token.DEFINE:
		j++
		if j < len(toks) && toks[j].tok == token.AND {
			j++
		}
	case i > 0 && toks[i-1].tok == token.VAR:
	default:
		return ""
	}
	if j+2 < len(toks) && toks[j].tok == token.IDENT && toks[j].lit == "port" && toks[j+1].tok == token.PERIOD {
		j += 2
	}
	if j < len(toks) && toks[j].tok == token.IDENT {
		return toks[j].lit
	}
	return ""
}

// rangeItemAt answers the item fields of the list x ranges over in
// `for _, x := range <chain>` with x at toks[i].
func rangeItemAt(toks []codeToken, i int, roots map[string][]normalizer.Field) []normalizer.Field {
	if i < 2 || toks[i-1].tok != token.COMMA || i+3 >= len(toks) {
		return nil
	}
	if (toks[i+1].tok != token.DEFINE && toks[i+1].tok != token.ASSIGN) || toks[i+2].tok != token.RANGE {
		return nil
	}
	root := toks[i+3]
	if root.tok != token.IDENT || roots[root.lit] == nil {
		return nil
	}
	fields := roots[root.lit]
	var list *normalizer.Field
	for _, sel := range selectorChain(toks, i+4) {
		list = nil
		f := fieldByGoName(fields, sel)
		if f == nil || len(f.ItemFields) == 0 {
			return nil
		}
		fields = f.ItemFields
		list = f
	}
	if list == nil || !isObjectList(*list) {
		return nil
	}
	return list.ItemFields
}

// selectorChain reads .A[i].B.C from toks[j]: the selected names, indexes skipped.
func selectorChain(toks []codeToken, j int) []string {
	var chain []string
	for j < len(toks) {
		switch {
		case toks[j].tok == token.PERIOD && j+1 < len(toks) && toks[j+1].tok == token.IDENT:
			chain = append(chain, toks[j+1].lit)
			j += 2
		case toks[j].tok == token.LBRACK:
			depth := 0
			for ; j < len(toks); j++ {
				if toks[j].tok == token.LBRACK {
					depth++
				} else if toks[j].tok == token.RBRACK {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			j++
		default:
			return chain
		}
	}
	return chain
}

// walkSelectorChain follows chain through fields and marks every loose object
// one of whose fields the chain selects.
func walkSelectorChain(fields []normalizer.Field, chain []string, reached map[string]bool) {
	for k, sel := range chain {
		f := fieldByGoName(fields, sel)
		if f == nil {
			return
		}
		switch {
		case isLooseObject(*f):
			if k+1 >= len(chain) || fieldByGoName(f.ItemFields, chain[k+1]) == nil {
				return
			}
			reached[f.ItemTypeName] = true
		case isObjectList(*f):
		default:
			return
		}
		fields = f.ItemFields
	}
}

func fieldByGoName(fields []normalizer.Field, goName string) *normalizer.Field {
	for i := range fields {
		if names.ToGoName(fields[i].Name) == goName {
			return &fields[i]
		}
	}
	return nil
}
