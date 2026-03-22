package emitter

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderFlowStepControlCollections(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "value.Coalesce":
		output := arg("output")
		if output == "" {
			return renderInvalidFlowStepConfig(st, pad, "value.Coalesce", "value.Coalesce requires output"), true
		}
		mode := strings.TrimSpace(arg("mode"))
		if mode == "" {
			mode = "non_empty"
		}
		var values []string
		switch raw := step.Args["values"].(type) {
		case []string:
			values = append(values, raw...)
		case []any:
			for _, it := range raw {
				if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
					values = append(values, normalizeFlowExpr(strings.TrimSpace(s)))
				}
			}
		case string:
			if strings.TrimSpace(raw) != "" {
				values = append(values, normalizeFlowExpr(strings.TrimSpace(raw)))
			}
		}
		if len(values) == 0 {
			return renderInvalidFlowStepConfig(st, pad, "value.Coalesce", "value.Coalesce requires values and output"), true
		}
		outputType := resolveFlowDynamicOutputType(st, output, arg("into"))
		foundV := "_coalesceFound" + sfx
		anyV := "_coalesceValue" + sfx
		modeV := "_coalesceMode" + sfx
		typedV := "_coalesceTyped" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%q))\n", pad, modeV, mode))
		b.WriteString(fmt.Sprintf("%svar %s any\n", pad, anyV))
		b.WriteString(fmt.Sprintf("%s%s := false\n", pad, foundV))
		b.WriteString(fmt.Sprintf("%sfor _, _candidate := range []any{%s} {\n", pad, strings.Join(values, ", ")))
		b.WriteString(fmt.Sprintf("%s\tif _candidate == nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tcontinue\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_use := false\n", pad))
		b.WriteString(fmt.Sprintf("%s\tswitch _v := _candidate.(type) {\n", pad))
		b.WriteString(fmt.Sprintf("%s\tcase string:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tswitch %s {\n", pad, modeV))
		b.WriteString(fmt.Sprintf("%s\t\tcase \"non_nil\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t_use = true\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tcase \"non_zero\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t_use = _v != \"\"\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tdefault:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t_use = strings.TrimSpace(_v) != \"\"\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tcase time.Time:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_use = %s == \"non_nil\" || !_v.IsZero()\n", pad, modeV))
		b.WriteString(fmt.Sprintf("%s\tcase int:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_use = %s == \"non_nil\" || _v != 0\n", pad, modeV))
		b.WriteString(fmt.Sprintf("%s\tcase int64:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_use = %s == \"non_nil\" || _v != 0\n", pad, modeV))
		b.WriteString(fmt.Sprintf("%s\tcase float64:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_use = %s == \"non_nil\" || _v != 0\n", pad, modeV))
		b.WriteString(fmt.Sprintf("%s\tcase bool:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_use = %s == \"non_nil\" || _v\n", pad, modeV))
		b.WriteString(fmt.Sprintf("%s\tdefault:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_use = true\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _use {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s = _candidate\n", pad, anyV))
		b.WriteString(fmt.Sprintf("%s\t\t%s = true\n", pad, foundV))
		b.WriteString(fmt.Sprintf("%s\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if outputType == "any" {
			b.WriteString(renderFlowAssignTarget(st, pad, output, anyV, "any"))
			return b.String(), true
		}
		if !st.declared[output] && !flowIsAssignableTarget(output) {
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = outputType
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, outputType))
		}
		b.WriteString(fmt.Sprintf("%sif %s {\n", pad, foundV))
		b.WriteString(fmt.Sprintf("%s\t%s, _ok := %s.(%s)\n", pad, typedV, anyV, outputType))
		b.WriteString(fmt.Sprintf("%s\tif !_ok {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"value.Coalesce: result is not %s\")", outputType)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad+"\t", output, typedV, outputType))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

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
			return renderInvalidFlowStepConfig(st, pad, "list.Append", "list.Append requires to and item"), true
		}
		return fmt.Sprintf("%s%s = append(%s, %s)\n", pad, to, to, item), true

	case "list.Find":
		from := arg("from")
		cond := arg("condition")
		output := arg("output")
		if from == "" || cond == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "list.Find", "list.Find requires from, condition, and output"), true
		}
		as := arg("as")
		if as == "" {
			as = "_item"
		}
		found := arg("found")
		outputType := resolveFlowDynamicOutputType(st, output, arg("into"))
		var b strings.Builder
		if found != "" {
			b.WriteString(renderFlowAssignTarget(st, pad, found, "false", "bool"))
		}
		if outputType != "any" && !st.declared[output] && !flowIsAssignableTarget(output) {
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = outputType
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, outputType))
		}
		b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, from))
		b.WriteString(fmt.Sprintf("%s\tif %s {\n", pad, cond))
		if found != "" {
			b.WriteString(fmt.Sprintf("%s\t\t%s = true\n", pad, found))
		}
		b.WriteString(renderFlowAssignTarget(st, pad+"\t\t", output, as, outputType))
		b.WriteString(fmt.Sprintf("%s\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "list.Any":
		from := arg("from")
		cond := arg("condition")
		output := arg("output")
		if from == "" || cond == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "list.Any", "list.Any requires from, condition, and output"), true
		}
		as := arg("as")
		if as == "" {
			as = "_item"
		}
		var b strings.Builder
		b.WriteString(renderFlowAssignTarget(st, pad, output, "false", "bool"))
		b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, from))
		b.WriteString(fmt.Sprintf("%s\tif %s {\n", pad, cond))
		b.WriteString(fmt.Sprintf("%s\t\t%s = true\n", pad, output))
		b.WriteString(fmt.Sprintf("%s\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "list.All":
		from := arg("from")
		cond := arg("condition")
		output := arg("output")
		if from == "" || cond == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "list.All", "list.All requires from, condition, and output"), true
		}
		as := arg("as")
		if as == "" {
			as = "_item"
		}
		var b strings.Builder
		b.WriteString(renderFlowAssignTarget(st, pad, output, "true", "bool"))
		b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, from))
		b.WriteString(fmt.Sprintf("%s\tif !(%s) {\n", pad, cond))
		b.WriteString(fmt.Sprintf("%s\t\t%s = false\n", pad, output))
		b.WriteString(fmt.Sprintf("%s\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "list.Map":
		return renderListMapLegacy(st, step, pad, arg), true

	case "list.Reduce":
		return renderListReduceLegacy(st, step, pad, sfx, arg), true

	case "list.GroupBy":
		return renderListGroupByLegacy(st, step, pad, sfx, arg), true

	case "list.Distinct":
		return renderListDistinctLegacy(st, step, pad, sfx, arg), true

	case "list.Chunk":
		return renderListChunkLegacy(st, step, pad, sfx, arg), true

	case "batch.Run":
		return renderBatchRunLegacy(st, step, pad, indent, sfx, arg, child), true

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

	case "list.Sum":
		input := arg("input")
		field := arg("field")
		output := arg("output")
		if input == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "list.Sum", "list.Sum requires input and output"), true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		var b strings.Builder
		tmp := "_sum" + sfx
		b.WriteString(fmt.Sprintf("%svar %s float64\n", pad, tmp))
		if field != "" {
			b.WriteString(fmt.Sprintf("%sfor _, _item := range %s { %s += float64(_item.%s) }\n", pad, input, tmp, field))
		} else {
			b.WriteString(fmt.Sprintf("%sfor _, _item := range %s { %s += float64(_item) }\n", pad, input, tmp))
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, tmp))
		return b.String(), true

	case "list.Avg":
		input := arg("input")
		field := arg("field")
		output := arg("output")
		if input == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "list.Avg", "list.Avg requires input and output"), true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		var b strings.Builder
		tmpSum := "_avgSum" + sfx
		b.WriteString(fmt.Sprintf("%svar %s float64\n", pad, tmpSum))
		if field != "" {
			b.WriteString(fmt.Sprintf("%sfor _, _item := range %s { %s += float64(_item.%s) }\n", pad, input, tmpSum, field))
		} else {
			b.WriteString(fmt.Sprintf("%sfor _, _item := range %s { %s += float64(_item) }\n", pad, input, tmpSum))
		}
		b.WriteString(fmt.Sprintf("%s%s %s 0.0\n", pad, output, assign))
		b.WriteString(fmt.Sprintf("%sif len(%s) > 0 { %s = %s / float64(len(%s)) }\n", pad, input, output, tmpSum, input))
		return b.String(), true
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
	// For time.Time fields (suffix At/Time/Date), use .Before()/.After() instead of < />
	isTimeSuffix := strings.HasSuffix(by, "At") || strings.HasSuffix(by, "Time") || strings.HasSuffix(by, "Date")
	var ascCmp, descCmp ast.Expr
	if isTimeSuffix {
		ascCmp = &ast.CallExpr{Fun: &ast.SelectorExpr{X: leftExpr, Sel: ast.NewIdent("Before")}, Args: []ast.Expr{rightExpr}}
		descCmp = &ast.CallExpr{Fun: &ast.SelectorExpr{X: rightExpr, Sel: ast.NewIdent("Before")}, Args: []ast.Expr{leftExpr}}
	} else {
		ascCmp = &ast.BinaryExpr{X: leftExpr, Op: token.LSS, Y: rightExpr}
		descCmp = &ast.BinaryExpr{X: leftExpr, Op: token.GTR, Y: rightExpr}
	}

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

func flowStepExprOrInt(step normalizer.FlowStep, key string, arg func(string) string) string {
	if s := arg(key); s != "" {
		return s
	}
	v, ok := step.Args[key]
	if !ok {
		return ""
	}
	switch n := v.(type) {
	case int:
		return strconv.Itoa(n)
	case int64:
		return strconv.FormatInt(n, 10)
	case float64:
		return strconv.Itoa(int(n))
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return ""
		}
		return s
	default:
		return ""
	}
}

func renderListMapLegacy(st *flowRenderState, step normalizer.FlowStep, pad string, arg func(string) string) string {
	from := arg("from")
	as := arg("as")
	expr := arg("expr")
	out := arg("output")
	if from == "" || expr == "" || out == "" {
		return ""
	}
	if as == "" {
		as = "item"
	}

	var b strings.Builder
	if st.declared[out] {
		b.WriteString(fmt.Sprintf("%s%s = %s[:0]\n", pad, out, out))
	} else {
		b.WriteString(fmt.Sprintf("%s%s := make([]any, 0, len(%s))\n", pad, out, from))
		st.declared[out] = true
	}
	st.pointers[out] = false
	st.types[out] = "[]any"
	b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, from))
	b.WriteString(fmt.Sprintf("%s\t%s = append(%s, %s)\n", pad, out, out, expr))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderListReduceLegacy(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) string {
	from := arg("from")
	as := arg("as")
	expr := arg("expr")
	out := arg("output")
	if from == "" || expr == "" || out == "" {
		return ""
	}
	if as == "" {
		as = "item"
	}
	initial := arg("initial")
	if initial == "" {
		initial = arg("init")
	}
	if initial == "" {
		initial = arg("seed")
	}
	assign := ":="
	if st.declared[out] {
		assign = "="
	}
	st.declared[out] = true
	st.pointers[out] = false

	var b strings.Builder
	if initial != "" {
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, out, assign, initial))
		b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, from))
		b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, out, expr))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()
	}

	itemsVar := "_reduceItems" + sfx
	b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, itemsVar, from))
	b.WriteString(fmt.Sprintf("%sif len(%s) == 0 {\n", pad, itemsVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "EMPTY_REDUCE_INPUT", "list.Reduce requires non-empty input when initial is not set")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s %s %s[0]\n", pad, out, assign, itemsVar))
	b.WriteString(fmt.Sprintf("%sfor _, %s := range %s[1:] {\n", pad, as, itemsVar))
	b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, out, expr))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderListGroupByLegacy(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) string {
	from := arg("from")
	as := arg("as")
	keyExpr := arg("key")
	out := arg("output")
	if from == "" || keyExpr == "" || out == "" {
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
	st.pointers[out] = false
	st.types[out] = "map[string][]any"

	keyVar := "_groupKey" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s %s make(map[string][]any)\n", pad, out, assign))
	b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, from))
	b.WriteString(fmt.Sprintf("%s\t%s := fmt.Sprint(%s)\n", pad, keyVar, keyExpr))
	b.WriteString(fmt.Sprintf("%s\t%s[%s] = append(%s[%s], %s)\n", pad, out, keyVar, out, keyVar, as))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderListDistinctLegacy(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) string {
	from := arg("from")
	as := arg("as")
	out := arg("output")
	keyExpr := arg("key")
	if from == "" || out == "" {
		return ""
	}
	if as == "" {
		as = "item"
	}
	if keyExpr == "" {
		keyExpr = as
	}
	assign := ":="
	if st.declared[out] {
		assign = "="
	}
	st.declared[out] = true

	seenVar := "_seen" + sfx
	keyVar := "_distinctKey" + sfx
	var b strings.Builder
	if assign == ":=" {
		b.WriteString(fmt.Sprintf("%s%s %s %s[:0:0]\n", pad, out, assign, from))
	} else {
		b.WriteString(fmt.Sprintf("%s%s = %s[:0]\n", pad, out, out))
	}
	b.WriteString(fmt.Sprintf("%s%s := map[string]bool{}\n", pad, seenVar))
	b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, from))
	b.WriteString(fmt.Sprintf("%s\t%s := fmt.Sprint(%s)\n", pad, keyVar, keyExpr))
	b.WriteString(fmt.Sprintf("%s\tif !%s[%s] {\n", pad, seenVar, keyVar))
	b.WriteString(fmt.Sprintf("%s\t\t%s[%s] = true\n", pad, seenVar, keyVar))
	b.WriteString(fmt.Sprintf("%s\t\t%s = append(%s, %s)\n", pad, out, out, as))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderListChunkLegacy(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) string {
	from := arg("from")
	out := arg("output")
	sizeExpr := flowStepExprOrInt(step, "size", arg)
	if from == "" || out == "" || sizeExpr == "" {
		return ""
	}
	assign := ":="
	if st.declared[out] {
		assign = "="
	}
	st.declared[out] = true
	st.pointers[out] = false
	st.types[out] = "[][]any"

	sizeVar := "_chunkSize" + sfx
	startVar := "_chunkStart" + sfx
	endVar := "_chunkEnd" + sfx
	itemVar := "_chunkItem" + sfx
	chunkVar := "_chunk" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, sizeVar, sizeExpr))
	b.WriteString(fmt.Sprintf("%sif %s <= 0 {\n", pad, sizeVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "INVALID_SIZE", "list.Chunk size must be > 0")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s %s make([][]any, 0)\n", pad, out, assign))
	b.WriteString(fmt.Sprintf("%sfor %s := 0; %s < len(%s); %s += %s {\n", pad, startVar, startVar, from, startVar, sizeVar))
	b.WriteString(fmt.Sprintf("%s\t%s := %s + %s\n", pad, endVar, startVar, sizeVar))
	b.WriteString(fmt.Sprintf("%s\tif %s > len(%s) {\n", pad, endVar, from))
	b.WriteString(fmt.Sprintf("%s\t\t%s = len(%s)\n", pad, endVar, from))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := make([]any, 0, %s-%s)\n", pad, chunkVar, endVar, startVar))
	b.WriteString(fmt.Sprintf("%s\tfor _, %s := range %s[%s:%s] {\n", pad, itemVar, from, startVar, endVar))
	b.WriteString(fmt.Sprintf("%s\t\t%s = append(%s, %s)\n", pad, chunkVar, chunkVar, itemVar))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s = append(%s, %s)\n", pad, out, out, chunkVar))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderBatchRunLegacy(st *flowRenderState, step normalizer.FlowStep, pad string, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	from := arg("from")
	if from == "" {
		return ""
	}
	as := arg("as")
	if as == "" {
		as = "batch"
	}
	sizeExpr := flowStepExprOrInt(step, "size", arg)
	if sizeExpr == "" {
		sizeExpr = "100"
	}
	doSteps := child("_do")
	if len(doSteps) == 0 {
		return ""
	}

	sizeVar := "_batchSize" + sfx
	startVar := "_batchStart" + sfx
	endVar := "_batchEnd" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, sizeVar, sizeExpr))
	b.WriteString(fmt.Sprintf("%sif %s <= 0 {\n", pad, sizeVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "INVALID_SIZE", "batch.Run size must be > 0")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sfor %s := 0; %s < len(%s); %s += %s {\n", pad, startVar, startVar, from, startVar, sizeVar))
	b.WriteString(fmt.Sprintf("%s\t%s := %s + %s\n", pad, endVar, startVar, sizeVar))
	b.WriteString(fmt.Sprintf("%s\tif %s > len(%s) {\n", pad, endVar, from))
	b.WriteString(fmt.Sprintf("%s\t\t%s = len(%s)\n", pad, endVar, from))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := %s[%s:%s]\n", pad, as, from, startVar, endVar))
	b.WriteString(renderFlowSteps(cloneFlowState(st), doSteps, indent+1))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
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
