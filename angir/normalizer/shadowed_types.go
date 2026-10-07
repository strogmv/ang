package normalizer

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
)

// predeclaredTypes are CUE's predeclared type identifiers that a field label
// can shadow.
var predeclaredTypes = map[string]bool{
	"number": true, "int": true, "float": true, "string": true, "bool": true, "bytes": true,
	"int8": true, "int16": true, "int32": true, "int64": true, "int128": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uint128": true,
	"float32": true, "float64": true, "rune": true, "null": true,
}

// checkShadowedType stops a field whose type is a predeclared identifier that
// a field of the same name in scope shadows: next to `number: string` (an
// invoice number) a bare `vatRatePercent: number` is that string field, not
// the type, and the API silently answers with a string.
func checkShadowedType(label string, v cue.Value) error {
	node := v.Source()
	if field, ok := node.(*ast.Field); ok {
		node = field.Value
	}
	if node == nil {
		return nil
	}
	var shadowed string
	ast.Walk(node, func(n ast.Node) bool {
		if shadowed != "" {
			return false
		}
		switch n := n.(type) {
		case *ast.StructLit, *ast.ListLit:
			// Nested fields are checked on their own.
			return n == node
		case *ast.Ident:
			if predeclaredTypes[n.Name] && n.Node != nil {
				shadowed = n.Name
			}
		}
		return true
	}, nil)
	if shadowed == "" {
		return nil
	}
	return fmt.Errorf("%s: field %q is typed %s, but a field named %q in scope shadows the CUE type, so it takes that field's value; write __%s (or a concrete type such as float64) or rename the other field",
		formatPos(v), label, shadowed, shadowed, shadowed)
}
