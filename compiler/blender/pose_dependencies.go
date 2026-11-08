package blender

import (
	"strconv"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
)

// poseScope is the actual instantiation of a generic CUE rule. Its parameters
// can have different values from the rule's source schema (floor, for example).
type poseScope struct {
	root cue.Value
	path cue.Path
}

func posePackageRoots(values ...cue.Value) []cue.Value {
	var roots []cue.Value
	context := cuecontext.New()
	seen := map[*build.Instance]bool{}
	var visit func(cue.Value, *build.Instance)
	visit = func(v cue.Value, instance *build.Instance) {
		if instance == nil || seen[instance] {
			return
		}
		seen[instance] = true
		roots = append(roots, v)
		for _, imported := range instance.Imports {
			if len(imported.Files) != 0 {
				visit(context.BuildInstance(imported), imported)
			}
		}
	}
	for _, v := range values {
		visit(v, v.BuildInstance())
	}
	return roots
}

func (g poseSourceGuard) scopeFor(v cue.Value, fallback poseScope) poseScope {
	selectors := v.Path().Selectors()
	if len(selectors) == 0 || selectors[0].LabelType() == cue.IndexLabel {
		return fallback // a builtin result is rootless, e.g. list.Range(...)[0]
	}
	for _, root := range g.roots {
		candidate := root.LookupPath(v.Path())
		if candidate.Exists() && candidate.Err() == nil && candidate.Pos().Filename() == v.Pos().Filename() && candidate.Pos().Line() == v.Pos().Line() && candidate.Pos().Column() == v.Pos().Column() {
			return poseScope{root: root, path: v.Path()}
		}
	}
	return fallback
}

func (g poseSourceGuard) scopedIdentifier(id *ast.Ident, scope poseScope) cue.Value {
	if !scope.root.Exists() {
		return cue.Value{}
	}
	selector := cue.Str(id.Name)
	if len(id.Name) > 0 && id.Name[0] == '_' {
		if instance := scope.root.BuildInstance(); instance != nil {
			selector = cue.Hid(id.Name, instance.ImportPath)
		}
	}
	ancestors := scope.path.Selectors()
	for n := len(ancestors) - 1; n >= 0; n-- {
		path := cue.MakePath(ancestors[:n]...).Append(selector)
		candidate := scope.root.LookupPath(path)
		if candidate.Exists() && candidate.Err() == nil {
			return candidate
		}
	}
	return cue.Value{}
}

// syntaxOrigin is only needed for scalars returned by a builtin, where CUE's
// public Expr API has replaced the operation by its result. Source preserves
// the original call and resolved ForClause/LetClause bindings. Follow those
// bindings, then resolve rule parameters in the actual instantiated scope.
func (g poseSourceGuard) syntaxOrigin(node ast.Node, scope poseScope, depth int, visiting map[ast.Node]bool) (bool, error) {
	if node == nil || depth > 100 || visiting[node] {
		return false, nil
	}
	visiting[node] = true
	defer delete(visiting, node)
	origin := func(n ast.Node) (bool, error) { return g.syntaxOrigin(n, scope, depth+1, visiting) }
	anyOrigin := func(nodes ...ast.Node) (bool, error) {
		found := false
		for _, n := range nodes {
			owned, err := origin(n)
			if err != nil {
				return false, err
			}
			found = found || owned
		}
		return found, nil
	}
	switch n := node.(type) {
	case *ast.Ident:
		if clause, ok := n.Scope.(*ast.ForClause); ok {
			return origin(clause.Source)
		}
		if clause, ok := n.Node.(*ast.LetClause); ok {
			return origin(clause.Expr)
		}
		if value := g.scopedIdentifier(n, scope); value.Exists() {
			key := value.Pos().Filename() + ":" + value.Path().String()
			if owned, ok := g.dependencies[key]; ok {
				return owned, nil
			}
			owned, err := g.valueDependency(value, scope, depth+1)
			if err != nil {
				return false, err
			}
			g.dependencies[key] = owned
			return owned, nil
		}
		if n.Node != n {
			return origin(n.Node)
		}
	case *ast.SelectorExpr:
		// Imported source data can also be a direct builtin argument.
		if id, ok := n.X.(*ast.Ident); ok {
			if imported, ok := id.Node.(*ast.ImportSpec); ok {
				path, _ := strconv.Unquote(imported.Path.Value)
				name, _, _ := ast.LabelName(n.Sel)
				for _, root := range g.roots {
					if instance := root.BuildInstance(); instance != nil && instance.ImportPath == path {
						value := root.LookupPath(cue.MakePath(cue.Str(name)))
						return g.valueDependency(value, scope, depth+1)
					}
				}
			}
		}
		return origin(n.X)
	case *ast.CallExpr:
		var nodes []ast.Node
		for _, argument := range n.Args {
			nodes = append(nodes, argument)
		}
		return anyOrigin(nodes...)
	case *ast.BinaryExpr:
		return anyOrigin(n.X, n.Y)
	case *ast.UnaryExpr:
		return origin(n.X)
	case *ast.ParenExpr:
		return origin(n.X)
	case *ast.IndexExpr:
		return anyOrigin(n.X, n.Index)
	case *ast.SliceExpr:
		return anyOrigin(n.X, n.Low, n.High)
	case *ast.Field:
		return origin(n.Value)
	case *ast.LetClause:
		return origin(n.Expr)
	case *ast.Alias:
		return origin(n.Expr)
	case *ast.Comprehension:
		return origin(n.Value)
	case *ast.ListLit:
		var nodes []ast.Node
		for _, element := range n.Elts {
			nodes = append(nodes, element)
		}
		return anyOrigin(nodes...)
	case *ast.StructLit:
		var nodes []ast.Node
		for _, field := range n.Elts {
			if f, ok := field.(*ast.Field); ok {
				nodes = append(nodes, f.Value)
			}
		}
		return anyOrigin(nodes...)
	}
	return false, nil
}

// A rule parameter can contain imported positions alongside fixed widths,
// heights or unit conversion factors. Track the imported inputs without
// misclassifying those semantic constants as independent coordinate owners.
// Numeric conjunctions retain numberOriginIn's mixed-source rejection.
func (g poseSourceGuard) valueDependency(v cue.Value, scope poseScope, depth int) (bool, error) {
	if depth > 100 || !v.Exists() || poseSchema(v) {
		return false, nil
	}
	if v.Kind()&cue.NumberKind != 0 {
		return g.numberOriginIn(v, true, depth+1, scope)
	}
	scope = g.scopeFor(v, scope)
	found := g.fromGenerated(v)
	if values, err := v.List(); err == nil {
		for values.Next() {
			owned, e := g.valueDependency(values.Value(), scope, depth+1)
			if e != nil {
				return false, e
			}
			found = found || owned
		}
	} else if fields, err := v.Fields(); err == nil {
		for fields.Next() {
			owned, e := g.valueDependency(fields.Value(), scope, depth+1)
			if e != nil {
				return false, e
			}
			found = found || owned
		}
	}
	return found, nil
}
