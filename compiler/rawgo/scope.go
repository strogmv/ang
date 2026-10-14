package rawgo

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"

	"golang.org/x/tools/go/ast/astutil"
)

// NameUse is a place in a snippet of raw Go where a name matters.
type NameUse struct {
	Name string
	Line int // line within the snippet, 1-based
	// DeclLine is the line of the snippet's own declaration of Name that is
	// in the way (ShadowedResults only).
	DeclLine int
	// Deferred is true for a deferred function literal that reads or sets
	// Name, false for a naked return (ShadowedResults only).
	Deferred bool
}

// UndeclaredUses returns the first use of each of names in a function literal
// (a logic.Call lambda) that the literal does not declare: such a use binds
// to whatever the generated method declares around the literal, such as the
// method's err. Field names, composite-literal keys and labels are not uses.
// ok is false when the literal does not parse.
func UndeclaredUses(literal string, names ...string) (uses []NameUse, ok bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "rawgo.go", "package rawgo\nvar _ = "+literal+"\n", 0)
	if err != nil {
		return nil, false
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	seen := map[string]bool{}
	var walk func(ast.Node)
	walk = func(root ast.Node) {
		ast.Inspect(root, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.SelectorExpr:
				walk(n.X) // n.Sel is a field or method name
				return false
			case *ast.KeyValueExpr:
				if _, isIdent := n.Key.(*ast.Ident); !isIdent {
					walk(n.Key)
				}
				walk(n.Value)
				return false
			case *ast.Field:
				if n.Type != nil {
					walk(n.Type) // n.Names declare
				}
				return false
			case *ast.LabeledStmt:
				walk(n.Stmt)
				return false
			case *ast.BranchStmt:
				return false
			case *ast.Ident:
				if n.Obj == nil && want[n.Name] && !seen[n.Name] {
					seen[n.Name] = true
					uses = append(uses, NameUse{Name: n.Name, Line: fset.Position(n.Pos()).Line - 1})
				}
			}
			return true
		})
	}
	walk(file)
	sort.Slice(uses, func(i, j int) bool { return uses[i].Line < uses[j].Line })
	return uses, true
}

// ShadowedResults reads code that runs as a block inside a method whose
// named results are resp and err (`impls: go:` code). The code may declare
// its own resp and err; they shadow the results, as in any nested scope. It
// returns the two places where that changes what the code means:
//   - a naked return where the code's own resp or err is in scope: Go
//     refuses it ("result parameter err not in scope at return");
//   - a deferred function literal that reads or sets the code's own resp or
//     err: it sees that variable, not the result the method returns.
//
// ok is false when the code does not parse.
func ShadowedResults(code string) (uses []NameUse, ok bool) {
	const header = 2
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "rawgo.go", "package rawgo\nfunc _() {\n"+code+"\n}\n", 0)
	if err != nil || len(file.Decls) == 0 {
		return nil, false
	}
	body := file.Decls[0].(*ast.FuncDecl).Body
	line := func(pos token.Pos) int { return fset.Position(pos).Line - header }

	// The code's own declarations of resp and err outside function
	// literals, with the scope each one is visible in.
	type decl struct {
		name  string
		pos   token.Pos
		scope ast.Node
	}
	var decls []decl
	ast.Inspect(body, func(node ast.Node) bool {
		if _, isLit := node.(*ast.FuncLit); isLit {
			return false
		}
		id, ok := node.(*ast.Ident)
		if !ok || (id.Name != "resp" && id.Name != "err") || id.Obj == nil || id.Obj.Pos() != id.Pos() {
			return true
		}
		if scope := declScope(file, id); scope != nil {
			decls = append(decls, decl{name: id.Name, pos: id.Pos(), scope: scope})
		}
		return true
	})
	if len(decls) == 0 {
		return nil, true
	}
	inScope := func(pos token.Pos) (decl, bool) {
		var best decl
		found := false
		for _, d := range decls {
			if d.pos < pos && d.scope.Pos() <= pos && pos < d.scope.End() && (!found || d.pos > best.pos) {
				best, found = d, true
			}
		}
		return best, found
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(n.Results) == 0 {
				if d, ok := inScope(n.Pos()); ok {
					uses = append(uses, NameUse{Name: d.name, Line: line(n.Pos()), DeclLine: line(d.pos)})
				}
			}
		case *ast.DeferStmt:
			lit, isLit := n.Call.Fun.(*ast.FuncLit)
			if !isLit {
				return true
			}
			reported := map[string]bool{}
			ast.Inspect(lit.Body, func(inner ast.Node) bool {
				id, ok := inner.(*ast.Ident)
				if !ok || reported[id.Name] || id.Obj == nil || (id.Name != "resp" && id.Name != "err") {
					return true
				}
				for _, d := range decls {
					if id.Obj.Pos() == d.pos {
						reported[id.Name] = true
						uses = append(uses, NameUse{Name: id.Name, Line: line(id.Pos()), DeclLine: line(d.pos), Deferred: true})
					}
				}
				return true
			})
			return false
		}
		return true
	})
	sort.Slice(uses, func(i, j int) bool { return uses[i].Line < uses[j].Line })
	return uses, true
}

// declScope is the node whose extent a local declared at id is visible in:
// the innermost block, clause or statement (if, for, switch) that encloses
// the declaration.
func declScope(file *ast.File, id *ast.Ident) ast.Node {
	path, _ := astutil.PathEnclosingInterval(file, id.Pos(), id.End())
	for _, node := range path[1:] {
		switch node.(type) {
		case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause, *ast.IfStmt, *ast.ForStmt,
			*ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.FuncLit:
			return node
		}
	}
	return nil
}
