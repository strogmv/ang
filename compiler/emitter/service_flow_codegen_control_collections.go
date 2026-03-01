package emitter

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepControlCollections(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, _ func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "list.Filter":
		if out, ok := renderListFilterAST(st, step, indent, arg); ok {
			return out, true
		}
		return renderListFilterLegacy(st, step, pad, arg), true

	case "list.Paginate":
		if out, ok := renderListPaginateAST(st, step, indent, sfx, arg); ok {
			return out, true
		}
		return renderListPaginateLegacy(st, step, pad, sfx, arg), true

	case "list.Append":
		to := arg("to")
		item := arg("item")
		if to == "" || item == "" {
			return "", true
		}
		return fmt.Sprintf("%s%s = append(%s, %s)\n", pad, to, to, item), true

	case "list.Sort":
		if out, ok := renderListSortAST(step, indent, arg); ok {
			return out, true
		}
		return renderListSortLegacy(pad, arg), true

	case "str.Normalize":
		if out, ok := renderStrNormalizeAST(st, step, indent, arg); ok {
			return out, true
		}
		return renderStrNormalizeLegacy(st, pad, arg), true
	}

	return "", false
}

func renderListSortAST(step normalizer.FlowStep, indent int, arg func(string) string) (string, bool) {
	items := arg("items")
	by := arg("by")
	order := arg("order") // raw: "asc" | "desc" | runtime expr e.g. "req.SortOrder"
	if items == "" || by == "" {
		return "", true
	}
	itemsExpr, err := parseFlowExprSafe(items)
	if err != nil {
		return "", false
	}
	leftExpr, err := parseFlowExprSafe(items + "[i]." + by)
	if err != nil {
		return "", false
	}
	rightExpr, err := parseFlowExprSafe(items + "[j]." + by)
	if err != nil {
		return "", false
	}
	ascCmp := &ast.BinaryExpr{X: leftExpr, Op: token.LSS, Y: rightExpr}
	descCmp := &ast.BinaryExpr{X: leftExpr, Op: token.GTR, Y: rightExpr}

	orderLower := strings.ToLower(order)
	isDynamic := order != "" && orderLower != "asc" && orderLower != "desc"

	fnBody := []ast.Stmt{}
	if isDynamic {
		orderExpr, parseErr := parseFlowExprSafe(order)
		if parseErr != nil {
			return "", false
		}
		fnBody = append(fnBody, &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X: &ast.CallExpr{
					Fun:  &ast.SelectorExpr{X: ast.NewIdent("strings"), Sel: ast.NewIdent("ToLower")},
					Args: []ast.Expr{orderExpr},
				},
				Op: token.EQL,
				Y:  &ast.BasicLit{Kind: token.STRING, Value: `"desc"`},
			},
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.ReturnStmt{Results: []ast.Expr{descCmp}},
			}},
		})
		fnBody = append(fnBody, &ast.ReturnStmt{Results: []ast.Expr{ascCmp}})
	} else {
		ret := ascCmp
		if orderLower == "desc" {
			ret = descCmp
		}
		fnBody = append(fnBody, &ast.ReturnStmt{Results: []ast.Expr{ret}})
	}

	sortStmt := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{X: ast.NewIdent("sort"), Sel: ast.NewIdent("Slice")},
			Args: []ast.Expr{
				itemsExpr,
				&ast.FuncLit{
					Type: &ast.FuncType{
						Params: &ast.FieldList{
							List: []*ast.Field{
								{Names: []*ast.Ident{ast.NewIdent("i")}, Type: ast.NewIdent("int")},
								{Names: []*ast.Ident{ast.NewIdent("j")}, Type: ast.NewIdent("int")},
							},
						},
						Results: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent("bool")}}},
					},
					Body: &ast.BlockStmt{List: fnBody},
				},
			},
		},
	}
	return renderFlowASTStmts([]ast.Stmt{sortStmt}, indent), true
}

func renderStrNormalizeAST(st *flowRenderState, _ normalizer.FlowStep, indent int, arg func(string) string) (string, bool) {
	in := arg("input")
	mode := strings.ToLower(arg("mode"))
	out := arg("output")
	if in == "" || out == "" {
		return "", true
	}
	inExpr, err := parseFlowExprSafe(in)
	if err != nil {
		return "", false
	}
	outExpr, err := parseFlowExprSafe(out)
	if err != nil {
		return "", false
	}
	defineOut := !st.declared[out]
	if defineOut {
		if _, ok := outExpr.(*ast.Ident); !ok {
			return "", false
		}
	}
	assignTok := token.ASSIGN
	if defineOut {
		assignTok = token.DEFINE
	}
	st.declared[out] = true
	st.pointers[out] = false
	st.types[out] = "string"

	trimCall := &ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: ast.NewIdent("strings"), Sel: ast.NewIdent("TrimSpace")},
		Args: []ast.Expr{inExpr},
	}
	var rhs ast.Expr
	switch mode {
	case "trim":
		rhs = trimCall
	case "upper":
		rhs = &ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: ast.NewIdent("strings"), Sel: ast.NewIdent("ToUpper")},
			Args: []ast.Expr{trimCall},
		}
	default:
		rhs = &ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: ast.NewIdent("strings"), Sel: ast.NewIdent("ToLower")},
			Args: []ast.Expr{trimCall},
		}
	}
	stmt := &ast.AssignStmt{
		Lhs: []ast.Expr{outExpr},
		Tok: assignTok,
		Rhs: []ast.Expr{rhs},
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func renderListFilterAST(st *flowRenderState, step normalizer.FlowStep, indent int, arg func(string) string) (string, bool) {
	from := arg("from")
	as := arg("as")
	cond := arg("condition")
	out := arg("output")
	if from == "" || out == "" || cond == "" {
		return "", true
	}
	if as == "" {
		as = "item"
	}
	fromExpr, err := parseFlowExprSafe(from)
	if err != nil {
		return "", false
	}
	condExpr, err := parseFlowExprSafe(cond)
	if err != nil {
		return "", false
	}
	outExpr, err := parseFlowExprSafe(out)
	if err != nil {
		return "", false
	}
	asIdent := ast.NewIdent(as)
	defineOut := !st.declared[out]
	if defineOut {
		// := requires plain identifier on LHS; fallback keeps legacy behavior for odd expressions.
		if _, ok := outExpr.(*ast.Ident); !ok {
			return "", false
		}
	}
	st.declared[out] = true

	assignTok := token.ASSIGN
	if defineOut {
		assignTok = token.DEFINE
	}
	zero := &ast.BasicLit{Kind: token.INT, Value: "0"}
	// Use full-slice expression [:0:0] to allocate independent backing array and avoid aliasing.
	initOut := &ast.AssignStmt{
		Lhs: []ast.Expr{outExpr},
		Tok: assignTok,
		Rhs: []ast.Expr{
			&ast.SliceExpr{
				X:      fromExpr,
				Low:    nil,
				High:   zero,
				Max:    zero,
				Slice3: true,
			},
		},
	}
	appendStmt := &ast.AssignStmt{
		Lhs: []ast.Expr{outExpr},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun:  ast.NewIdent("append"),
				Args: []ast.Expr{outExpr, asIdent},
			},
		},
	}
	rangeStmt := &ast.RangeStmt{
		Key:   ast.NewIdent("_"),
		Value: asIdent,
		Tok:   token.DEFINE,
		X:     fromExpr,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: condExpr,
					Body: &ast.BlockStmt{List: []ast.Stmt{appendStmt}},
				},
			},
		},
	}
	return renderFlowASTStmts([]ast.Stmt{initOut, rangeStmt}, indent), true
}

func renderListPaginateAST(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string) (string, bool) {
	in := arg("input")
	off := arg("offset")
	lim := arg("limit")
	out := arg("output")
	if in == "" || off == "" || lim == "" || out == "" {
		return "", true
	}
	inExpr, err := parseFlowExprSafe(in)
	if err != nil {
		return "", false
	}
	offExpr, err := parseFlowExprSafe(off)
	if err != nil {
		return "", false
	}
	limExpr, err := parseFlowExprSafe(lim)
	if err != nil {
		return "", false
	}
	outExpr, err := parseFlowExprSafe(out)
	if err != nil {
		return "", false
	}
	defineOut := !st.declared[out]
	if defineOut {
		if _, ok := outExpr.(*ast.Ident); !ok {
			return "", false
		}
	}
	st.declared[out] = true
	st.pointers[out] = false

	defaultLimit := flowIntArg(step.Args, "defaultLimit", 50)
	ov, lv, sv, ev := "_off"+sfx, "_lim"+sfx, "_start"+sfx, "_end"+sfx
	zero := &ast.BasicLit{Kind: token.INT, Value: "0"}
	defLimit := flowIntLit(defaultLimit)
	lenIn := func() ast.Expr {
		return &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{inExpr}}
	}
	assignOutTok := token.ASSIGN
	if defineOut {
		assignOutTok = token.DEFINE
	}
	// Dedicated temp vars with deterministic suffix avoid re-declaration clashes across steps.
	stmts := []ast.Stmt{
		&ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(ov)}, Tok: token.DEFINE, Rhs: []ast.Expr{offExpr}},
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: ast.NewIdent(ov), Op: token.LSS, Y: zero},
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(ov)}, Tok: token.ASSIGN, Rhs: []ast.Expr{zero}},
			}},
		},
		&ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(lv)}, Tok: token.DEFINE, Rhs: []ast.Expr{limExpr}},
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: ast.NewIdent(lv), Op: token.LEQ, Y: zero},
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(lv)}, Tok: token.ASSIGN, Rhs: []ast.Expr{defLimit}},
			}},
		},
		&ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(sv)}, Tok: token.DEFINE, Rhs: []ast.Expr{ast.NewIdent(ov)}},
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: ast.NewIdent(sv), Op: token.GTR, Y: lenIn()},
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(sv)}, Tok: token.ASSIGN, Rhs: []ast.Expr{lenIn()}},
			}},
		},
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(ev)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.BinaryExpr{X: ast.NewIdent(sv), Op: token.ADD, Y: ast.NewIdent(lv)}},
		},
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: ast.NewIdent(ev), Op: token.GTR, Y: lenIn()},
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(ev)}, Tok: token.ASSIGN, Rhs: []ast.Expr{lenIn()}},
			}},
		},
		&ast.AssignStmt{
			Lhs: []ast.Expr{outExpr},
			Tok: assignOutTok,
			Rhs: []ast.Expr{
				&ast.SliceExpr{
					X:    inExpr,
					Low:  ast.NewIdent(sv),
					High: ast.NewIdent(ev),
				},
			},
		},
	}
	return renderFlowASTStmts(stmts, indent), true
}

func renderListFilterLegacy(st *flowRenderState, step normalizer.FlowStep, pad string, arg func(string) string) string {
	from := arg("from")
	as := arg("as")
	cond := arg("condition")
	out := arg("output")
	if from == "" || out == "" || cond == "" {
		return ""
	}
	if as == "" {
		as = "item"
	}
	assign := ":="
	if st.declared[out] {
		assign = "="
	}
	st.declared[out] = true
	return fmt.Sprintf("%s%s %s %s[:0:0]\n%sfor _, %s := range %s {\n%s\tif %s {\n%s\t\t%s = append(%s, %s)\n%s\t}\n%s}\n",
		pad, out, assign, from,
		pad, as, from,
		pad, cond,
		pad, out, out, as,
		pad, pad)
}

func renderListPaginateLegacy(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) string {
	in := arg("input")
	off := arg("offset")
	lim := arg("limit")
	out := arg("output")
	if in == "" || off == "" || lim == "" || out == "" {
		return ""
	}
	assign := ":="
	if st.declared[out] {
		assign = "="
	}
	st.declared[out] = true
	st.pointers[out] = false
	defaultLimit := 50
	if v, ok := step.Args["defaultLimit"]; ok {
		switch n := v.(type) {
		case int:
			defaultLimit = n
		case int64:
			defaultLimit = int(n)
		case float64:
			defaultLimit = int(n)
		}
	}
	ov, lv, sv, ev := "_off"+sfx, "_lim"+sfx, "_start"+sfx, "_end"+sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, ov, off))
	b.WriteString(fmt.Sprintf("%sif %s < 0 { %s = 0 }\n", pad, ov, ov))
	b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, lv, lim))
	b.WriteString(fmt.Sprintf("%sif %s <= 0 { %s = %d }\n", pad, lv, lv, defaultLimit))
	b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, sv, ov))
	b.WriteString(fmt.Sprintf("%sif %s > len(%s) { %s = len(%s) }\n", pad, sv, in, sv, in))
	b.WriteString(fmt.Sprintf("%s%s := %s + %s\n", pad, ev, sv, lv))
	b.WriteString(fmt.Sprintf("%sif %s > len(%s) { %s = len(%s) }\n", pad, ev, in, ev, in))
	b.WriteString(fmt.Sprintf("%s%s %s %s[%s:%s]\n", pad, out, assign, in, sv, ev))
	return b.String()
}

func renderListSortLegacy(pad string, arg func(string) string) string {
	items := arg("items")
	by := arg("by")
	order := arg("order")
	if items == "" || by == "" {
		return ""
	}
	var b strings.Builder
	orderLower := strings.ToLower(order)
	isDynamic := order != "" && orderLower != "asc" && orderLower != "desc"
	if isDynamic {
		b.WriteString(fmt.Sprintf("%ssort.Slice(%s, func(i, j int) bool {\n", pad, items))
		b.WriteString(fmt.Sprintf("%s\tif strings.ToLower(%s) == \"desc\" { return %s[i].%s > %s[j].%s }\n", pad, order, items, by, items, by))
		b.WriteString(fmt.Sprintf("%s\treturn %s[i].%s < %s[j].%s\n", pad, items, by, items, by))
		b.WriteString(fmt.Sprintf("%s})\n", pad))
	} else {
		cmp := "<"
		if orderLower == "desc" {
			cmp = ">"
		}
		b.WriteString(fmt.Sprintf("%ssort.Slice(%s, func(i, j int) bool { return %s[i].%s %s %s[j].%s })\n", pad, items, items, by, cmp, items, by))
	}
	return b.String()
}

func renderStrNormalizeLegacy(st *flowRenderState, pad string, arg func(string) string) string {
	in := arg("input")
	mode := strings.ToLower(arg("mode"))
	out := arg("output")
	if in == "" || out == "" {
		return ""
	}
	assign := ":="
	if st.declared[out] {
		assign = "="
	}
	st.declared[out] = true
	st.pointers[out] = false
	st.types[out] = "string"
	switch mode {
	case "trim":
		return fmt.Sprintf("%s%s %s strings.TrimSpace(%s)\n", pad, out, assign, in)
	case "upper":
		return fmt.Sprintf("%s%s %s strings.ToUpper(strings.TrimSpace(%s))\n", pad, out, assign, in)
	default:
		return fmt.Sprintf("%s%s %s strings.ToLower(strings.TrimSpace(%s))\n", pad, out, assign, in)
	}
}
