package emitter

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderFlowStepControlFlow(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "flow.If":
		if out, ok := renderFlowIfAST(st, indent, arg, child); ok {
			return out, true
		}
		return renderFlowIfLegacy(st, pad, indent, arg, child), true

	case "flow.For":
		if out, ok := renderFlowForAST(st, indent, arg, child); ok {
			return out, true
		}
		return renderFlowForLegacy(st, pad, indent, arg, child), true

	case "flow.Block", "tx.Block":
		if out, ok := renderFlowBlockAST(st, indent, child); ok {
			return out, true
		}
		return renderFlowBlockLegacy(st, indent, child), true

	case "flow.Switch":
		if out, ok := renderFlowSwitchAST(st, step, indent, arg, child); ok {
			return out, true
		}
		return renderFlowSwitchLegacy(st, step, pad, indent, arg, child), true

	case "flow.While":
		if out, ok := renderFlowWhileAST(st, indent, arg, child); ok {
			return out, true
		}
		return renderFlowWhileLegacy(st, pad, indent, arg, child), true

	case "flow.Call":
		return renderFlowCall(st, pad, step, arg), true

	case "flow.Checkpoint":
		if out, ok := renderFlowCheckpointAST(indent, arg); ok {
			return out, true
		}
		return renderFlowCheckpointLegacy(pad, arg), true

	case "flow.Resume":
		if out, ok := renderFlowResumeAST(st, indent, sfx, arg, child); ok {
			return out, true
		}
		return renderFlowResumeLegacy(st, pad, indent, sfx, arg, child), true

	case "flow.RecordEvent":
		return renderFlowRecordEvent(st, indent, sfx, arg), true

	case "flow.History.Get":
		return renderFlowHistoryGet(st, indent, sfx, arg), true

	case "flow.Replay":
		return renderFlowReplay(st, indent, sfx, arg, child), true

	case "flow.Validate":
		if out, ok := renderFlowValidateAST(st, indent, arg); ok {
			return out, true
		}
		return renderFlowValidateLegacy(st, pad, arg), true

	case "flow.Catch":
		if out, ok := renderFlowCatchAST(st, indent, child); ok {
			return out, true
		}
		return renderFlowCatchLegacy(st, pad, indent, child), true

	case "flow.Defer":
		if out, ok := renderFlowDeferAST(st, indent, child); ok {
			return out, true
		}
		return renderFlowDeferLegacy(st, pad, indent, child), true

	case "flow.SuggestNext":
		if out, ok := renderFlowSuggestNextAST(st, step, indent, arg); ok {
			return out, true
		}
		return renderFlowSuggestNextLegacy(st, step, pad, arg), true

	case "flow.ExplainError":
		if out, ok := renderFlowExplainErrorAST(st, indent, sfx, arg); ok {
			return out, true
		}
		return renderFlowExplainErrorLegacy(st, pad, sfx, arg), true

	case "flow.Parallel":
		return renderFlowParallel(st, step, indent, sfx, arg, child), true

	case "flow.Join":
		return renderFlowJoin(st, step, indent, sfx, arg, child), true

	case "flow.Race":
		return renderFlowRace(st, step, indent, sfx, arg, child), true

	case "flow.Delay":
		return renderFlowDelay(st, step, indent, sfx, arg, child), true

	case "flow.Schedule":
		return renderFlowSchedule(st, step, indent, sfx, arg, child), true

	case "flow.Cron":
		return renderFlowCron(st, step, indent, sfx, arg, child), true

	case "flow.Tag":
		return renderFlowTag(st, step, indent, sfx, arg, child), true

	case "flow.Return":
		// Early return from the flow with the current response value.
		// Optional: set a field before returning:
		//   { action: "flow.Return", set: "resp.Status", value: `"ok"` }
		setField := arg("set")
		setValue := arg("value")
		var b strings.Builder
		if setField != "" && setValue != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, setField, setValue))
		}
		b.WriteString(returnSuccess(st, pad))
		return b.String(), true
	}

	return "", false
}

func renderFlowTag(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	pad := strings.Repeat("\t", indent)
	name := arg("name")
	value := arg("value")
	if name == "" {
		return ""
	}

	if value != "" {
		return fmt.Sprintf("%sslog.Info(\"flow.tag\", \"name\", %s, \"value\", %s)\n", pad, name, value)
	}
	return fmt.Sprintf("%sslog.Info(\"flow.tag\", \"name\", %s)\n", pad, name)
}

func renderFlowCall(st *flowRenderState, pad string, step normalizer.FlowStep, arg func(string) string) string {
	opRaw := strings.TrimSpace(arg("op"))
	if opRaw == "" {
		return ""
	}
	output := strings.TrimSpace(arg("output"))
	ignoreErr, _ := step.Args["ignoreErr"].(bool)

	serviceName := ""
	methodName := opRaw
	if parts := strings.SplitN(opRaw, ".", 2); len(parts) == 2 {
		serviceName = strings.TrimSpace(parts[0])
		methodName = strings.TrimSpace(parts[1])
	}
	if methodName == "" {
		return ""
	}
	methodExport := ExportName(methodName)

	reqExpr := fmt.Sprintf("port.%sRequest{}", methodExport)
	argMap := map[string]string{}
	switch raw := step.Args["args"].(type) {
	case map[string]string:
		argMap = raw
	case map[string]any:
		for k, v := range raw {
			argMap[k] = fmt.Sprint(v)
		}
	}
	if len(argMap) > 0 {
		keys := make([]string, 0, len(argMap))
		for k := range argMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fields := make([]string, 0, len(keys))
		for _, k := range keys {
			expr := normalizeFlowExpr(strings.TrimSpace(argMap[k]))
			if expr == "" {
				continue
			}
			fields = append(fields, fmt.Sprintf("%s: %s", ExportName(k), expr))
		}
		if len(fields) > 0 {
			reqExpr = fmt.Sprintf("port.%sRequest{%s}", methodExport, strings.Join(fields, ", "))
		}
	}

	callStr := fmt.Sprintf("s.%s(ctx, %s)", methodExport, reqExpr)
	if strings.TrimSpace(serviceName) != "" && !strings.EqualFold(strings.TrimSpace(serviceName), strings.TrimSpace(st.serviceName)) {
		callStr = fmt.Sprintf("s.%sService.%s(ctx, %s)", ExportName(serviceName), methodExport, reqExpr)
	}

	if ignoreErr {
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "port." + methodExport + "Response"
			return fmt.Sprintf("%s%s %s %s\n", pad, output+", _", assign, callStr)
		}
		return fmt.Sprintf("%s_, _ = %s\n", pad, callStr)
	}

	if output != "" {
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "port." + methodExport + "Response"
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output+", err", assign, callStr))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif _, err := %s; err != nil {\n", pad, callStr))
	b.WriteString(errReturn(st, pad+"\t", "err"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowIfAST(st *flowRenderState, indent int, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	cond := arg("condition")
	if cond == "" {
		return "", true
	}
	condExpr, err := parseFlowExprSafe(cond)
	if err != nil {
		return "", false
	}
	thenBody, err := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), child("_then"), 1))
	if err != nil {
		return "", false
	}
	stmt := &ast.IfStmt{
		Cond: condExpr,
		Body: &ast.BlockStmt{List: thenBody},
	}
	elseSteps := child("_else")
	if len(elseSteps) > 0 {
		elseBody, parseErr := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), elseSteps, 1))
		if parseErr != nil {
			return "", false
		}
		stmt.Else = &ast.BlockStmt{List: elseBody}
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func renderFlowBlockAST(st *flowRenderState, indent int, child func(string) []normalizer.FlowStep) (string, bool) {
	body, err := parseFlowStmtList(renderFlowSteps(st, child("_do"), 1))
	if err != nil {
		return "", false
	}
	return renderFlowASTStmts(body, indent), true
}

func renderFlowForAST(st *flowRenderState, indent int, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	each := arg("each")
	as := arg("as")
	if each == "" {
		return "", true
	}
	if as == "" {
		as = "item"
	}
	eachExpr, err := parseFlowExprSafe(each)
	if err != nil {
		return "", false
	}
	asExpr, err := parseFlowExprSafe(as)
	if err != nil {
		return "", false
	}
	asIdent, ok := asExpr.(*ast.Ident)
	if !ok {
		return "", false
	}
	body, err := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), child("_do"), 1))
	if err != nil {
		return "", false
	}
	stmt := &ast.RangeStmt{
		Key:   ast.NewIdent("_"),
		Value: asIdent,
		Tok:   token.DEFINE,
		X:     eachExpr,
		Body:  &ast.BlockStmt{List: body},
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func renderFlowSwitchAST(st *flowRenderState, step normalizer.FlowStep, indent int, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	value := arg("value")
	if value == "" {
		return "", true
	}
	valueExpr, err := parseFlowExprSafe(value)
	if err != nil {
		return "", false
	}
	cases, keys := flowSwitchCases(step)
	clauses := make([]ast.Stmt, 0, len(keys)+1)
	for _, k := range keys {
		body, parseErr := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), cases[k], 1))
		if parseErr != nil {
			return "", false
		}
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{
				&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", k)},
			},
			Body: body,
		})
	}
	defaultSteps := child("_default")
	if len(defaultSteps) > 0 {
		body, parseErr := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), defaultSteps, 1))
		if parseErr != nil {
			return "", false
		}
		clauses = append(clauses, &ast.CaseClause{Body: body})
	}
	stmt := &ast.SwitchStmt{
		Tag:  valueExpr,
		Body: &ast.BlockStmt{List: clauses},
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func renderFlowWhileAST(st *flowRenderState, indent int, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	cond := arg("condition")
	if cond == "" {
		return "", true
	}
	condExpr, err := parseFlowExprSafe(cond)
	if err != nil {
		return "", false
	}
	body, err := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), child("_do"), 1))
	if err != nil {
		return "", false
	}
	stmt := &ast.ForStmt{
		Cond: condExpr,
		Body: &ast.BlockStmt{List: body},
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func renderFlowCheckpointAST(indent int, arg func(string) string) (string, bool) {
	name := arg("name")
	if name == "" {
		return "", true
	}
	data := arg("data")
	if data == "" {
		data = "map[string]any{\"resp\": resp}"
	}
	dataExpr, err := parseFlowExprSafe(data)
	if err != nil {
		return "", false
	}
	keyExpr := &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", name)}
	stmts := []ast.Stmt{
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X:  ast.NewIdent("_flowCheckpoints"),
				Op: token.EQL,
				Y:  ast.NewIdent("nil"),
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{ast.NewIdent("_flowCheckpoints")},
						Tok: token.ASSIGN,
						Rhs: []ast.Expr{
							&ast.CallExpr{
								Fun: ast.NewIdent("make"),
								Args: []ast.Expr{
									&ast.MapType{Key: ast.NewIdent("string"), Value: ast.NewIdent("any")},
								},
							},
						},
					},
				},
			},
		},
		&ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.IndexExpr{
					X:     ast.NewIdent("_flowCheckpoints"),
					Index: keyExpr,
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{dataExpr},
		},
	}
	return renderFlowASTStmts(stmts, indent), true
}

func renderFlowResumeAST(st *flowRenderState, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	name := arg("name")
	if name == "" {
		return "", true
	}
	output := arg("output")
	onMissing := child("_onMissing")
	ckptValV, ckptOKV := "_ckptVal"+sfx, "_ckptOK"+sfx
	keyExpr := &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", name)}

	stmts := []ast.Stmt{
		&ast.DeclStmt{
			Decl: &ast.GenDecl{
				Tok: token.VAR,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names: []*ast.Ident{ast.NewIdent(ckptValV)},
						Type:  ast.NewIdent("any"),
					},
				},
			},
		},
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(ckptOKV)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{ast.NewIdent("false")},
		},
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X:  ast.NewIdent("_flowCheckpoints"),
				Op: token.NEQ,
				Y:  ast.NewIdent("nil"),
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{ast.NewIdent(ckptValV), ast.NewIdent(ckptOKV)},
						Tok: token.ASSIGN,
						Rhs: []ast.Expr{
							&ast.IndexExpr{
								X:     ast.NewIdent("_flowCheckpoints"),
								Index: keyExpr,
							},
						},
					},
				},
			},
		},
	}

	missingBody := []ast.Stmt{}
	if len(onMissing) > 0 {
		parsed, err := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), onMissing, 1))
		if err != nil {
			return "", false
		}
		missingBody = append(missingBody, parsed...)
	} else {
		parsed, err := parseFlowStmtList(errReturn(st, "", fmt.Sprintf("errors.New(http.StatusNotFound, \"CHECKPOINT_NOT_FOUND\", \"checkpoint %s not found\")", name)))
		if err != nil {
			return "", false
		}
		missingBody = append(missingBody, parsed...)
	}
	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.UnaryExpr{
			Op: token.NOT,
			X:  ast.NewIdent(ckptOKV),
		},
		Body: &ast.BlockStmt{List: missingBody},
	})

	if output != "" {
		outExpr, err := parseFlowExprSafe(output)
		if err != nil {
			return "", false
		}
		assignTok := token.ASSIGN
		if !st.declared[output] {
			if _, ok := outExpr.(*ast.Ident); !ok {
				return "", false
			}
			assignTok = token.DEFINE
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "any"
		stmts = append(stmts, &ast.AssignStmt{
			Lhs: []ast.Expr{outExpr},
			Tok: assignTok,
			Rhs: []ast.Expr{ast.NewIdent(ckptValV)},
		})
	}

	return renderFlowASTStmts(stmts, indent), true
}

func renderFlowCatchAST(st *flowRenderState, indent int, child func(string) []normalizer.FlowStep) (string, bool) {
	catchSteps := child("_do")
	if len(catchSteps) == 0 {
		return "", true
	}
	catchBody, err := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), catchSteps, 1))
	if err != nil {
		return "", false
	}
	catchBody = append(catchBody, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("_flowLastError")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent("nil")},
	})
	stmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent("_flowLastError"),
			Op: token.NEQ,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{List: catchBody},
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func renderFlowDeferAST(st *flowRenderState, indent int, child func(string) []normalizer.FlowStep) (string, bool) {
	deferSteps := child("_do")
	if len(deferSteps) == 0 {
		return "", true
	}
	pad := strings.Repeat("\t", indent)

	predecl := flowDeferPredeclaredStringVars(st, deferSteps)
	for _, name := range predecl {
		st.declared[name] = true
		st.pointers[name] = false
		st.types[name] = "string"
	}

	deferState := cloneFlowState(st)
	deferState.concurrMode = "race" // errReturn => plain `return` inside deferred closure
	body, err := parseFlowStmtList(renderFlowSteps(deferState, deferSteps, 1))
	if err != nil {
		return "", false
	}

	stmt := &ast.DeferStmt{
		Call: &ast.CallExpr{
			Fun: &ast.FuncLit{
				Type: &ast.FuncType{Params: &ast.FieldList{}},
				Body: &ast.BlockStmt{List: body},
			},
		},
	}

	var b strings.Builder
	for _, name := range predecl {
		b.WriteString(fmt.Sprintf("%svar %s string\n", pad, name))
	}
	b.WriteString(renderFlowASTStmts([]ast.Stmt{stmt}, indent))
	return b.String(), true
}

func renderFlowDeferLegacy(st *flowRenderState, pad string, indent int, child func(string) []normalizer.FlowStep) string {
	deferSteps := child("_do")
	if len(deferSteps) == 0 {
		return ""
	}
	predecl := flowDeferPredeclaredStringVars(st, deferSteps)
	for _, name := range predecl {
		st.declared[name] = true
		st.pointers[name] = false
		st.types[name] = "string"
	}

	var b strings.Builder
	for _, name := range predecl {
		b.WriteString(fmt.Sprintf("%svar %s string\n", pad, name))
	}
	b.WriteString(fmt.Sprintf("%sdefer func() {\n", pad))
	deferState := cloneFlowState(st)
	deferState.concurrMode = "race"
	b.WriteString(renderFlowSteps(deferState, deferSteps, indent+1))
	b.WriteString(fmt.Sprintf("%s}()\n", pad))
	return b.String()
}

func flowDeferPredeclaredStringVars(st *flowRenderState, steps []normalizer.FlowStep) []string {
	set := map[string]struct{}{}
	var walk func([]normalizer.FlowStep)
	walk = func(items []normalizer.FlowStep) {
		for _, s := range items {
			if s.Action == "fs.Remove" {
				if raw, ok := s.Args["path"].(string); ok {
					if ident, ok := flowSimpleIdent(normalizeFlowExpr(strings.TrimSpace(raw))); ok && !st.declared[ident] {
						set[ident] = struct{}{}
					}
				}
			}
			for _, nested := range flowChildSteps(s) {
				walk(nested)
			}
		}
	}
	walk(steps)
	if len(set) == 0 {
		return nil
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func flowSimpleIdent(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	for i, r := range s {
		if i == 0 {
			if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return "", false
			}
			continue
		}
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return "", false
		}
	}
	return s, true
}

func renderFlowSuggestNextAST(st *flowRenderState, step normalizer.FlowStep, indent int, arg func(string) string) (string, bool) {
	output := arg("output")
	options := flowSuggestNextOptions(step)
	if len(options) == 0 {
		return "", true
	}
	elts := make([]ast.Expr, 0, len(options))
	for _, opt := range options {
		elts = append(elts, &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", opt)})
	}
	listExpr := &ast.CompositeLit{
		Type: &ast.ArrayType{Elt: ast.NewIdent("string")},
		Elts: elts,
	}
	if output != "" {
		outExpr, err := parseFlowExprSafe(output)
		if err != nil {
			return "", false
		}
		assignTok := token.ASSIGN
		if !st.declared[output] {
			if _, ok := outExpr.(*ast.Ident); !ok {
				return "", false
			}
			assignTok = token.DEFINE
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "[]string"
		stmt := &ast.AssignStmt{
			Lhs: []ast.Expr{outExpr},
			Tok: assignTok,
			Rhs: []ast.Expr{listExpr},
		}
		return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
	}
	stmt := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{X: ast.NewIdent("slog"), Sel: ast.NewIdent("Info")},
			Args: []ast.Expr{
				&ast.BasicLit{Kind: token.STRING, Value: `"flow.suggest_next"`},
				&ast.BasicLit{Kind: token.STRING, Value: `"options"`},
				listExpr,
			},
		},
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func flowSuggestNextOptions(step normalizer.FlowStep) []string {
	var options []string
	v, ok := step.Args["options"]
	if !ok {
		return options
	}
	switch x := v.(type) {
	case []string:
		options = append(options, x...)
	case string:
		if strings.TrimSpace(x) != "" {
			options = []string{x}
		}
	}
	return options
}

func flowSwitchCases(step normalizer.FlowStep) (map[string][]normalizer.FlowStep, []string) {
	cases, _ := step.Args["_cases"].(map[string][]normalizer.FlowStep)
	keys := make([]string, 0, len(cases))
	for k := range cases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return cases, keys
}

func renderFlowValidateAST(st *flowRenderState, indent int, arg func(string) string) (string, bool) {
	cond := arg("condition")
	if cond == "" {
		return "", true
	}
	condExpr, err := parseFlowExprSafe(cond)
	if err != nil {
		return "", false
	}
	message := arg("message")
	if message == "" {
		message = arg("throw")
	}
	if message == "" {
		message = "validation failed"
	}
	if hint := arg("hint"); hint != "" {
		message = message + " (hint: " + hint + ")"
	}
	code := arg("code")
	if code == "" {
		code = "VALIDATION_FAILED"
	}
	status := arg("status")
	if status == "" {
		status = "http.StatusBadRequest"
	}
	body, parseErr := parseFlowStmtList(errReturn(st, "", fmt.Sprintf("errors.New(%s, %q, %q)", status, code, message)))
	if parseErr != nil {
		return "", false
	}
	stmt := &ast.IfStmt{
		Cond: &ast.UnaryExpr{
			Op: token.NOT,
			X:  condExpr,
		},
		Body: &ast.BlockStmt{List: body},
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func renderFlowExplainErrorAST(st *flowRenderState, indent int, sfx string, arg func(string) string) (string, bool) {
	errExprRaw := arg("error")
	if errExprRaw == "" {
		errExprRaw = "_flowLastError"
	}
	errExpr, err := parseFlowExprSafe(errExprRaw)
	if err != nil {
		return "", false
	}
	output := arg("output")
	message := arg("message")
	hint := arg("hint")
	expMsgV := "_expMsg" + sfx

	ifBody := []ast.Stmt{
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(expMsgV)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun: &ast.SelectorExpr{X: ast.NewIdent("fmt"), Sel: ast.NewIdent("Sprintf")},
					Args: []ast.Expr{
						&ast.BasicLit{Kind: token.STRING, Value: `"flow error: %v"`},
						errExpr,
					},
				},
			},
		},
	}
	if message != "" {
		ifBody = append(ifBody, &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(expMsgV)},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.BinaryExpr{
					X: &ast.BinaryExpr{
						X:  &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", message)},
						Op: token.ADD,
						Y:  &ast.BasicLit{Kind: token.STRING, Value: `": "`},
					},
					Op: token.ADD,
					Y:  ast.NewIdent(expMsgV),
				},
			},
		})
	}
	if hint != "" {
		ifBody = append(ifBody, &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(expMsgV)},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.BinaryExpr{
					X:  ast.NewIdent(expMsgV),
					Op: token.ADD,
					Y:  &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", " | hint: "+hint)},
				},
			},
		})
	}
	if output != "" {
		outExpr, parseErr := parseFlowExprSafe(output)
		if parseErr != nil {
			return "", false
		}
		assignTok := token.ASSIGN
		if !st.declared[output] {
			if _, ok := outExpr.(*ast.Ident); !ok {
				return "", false
			}
			assignTok = token.DEFINE
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		ifBody = append(ifBody, &ast.AssignStmt{
			Lhs: []ast.Expr{outExpr},
			Tok: assignTok,
			Rhs: []ast.Expr{ast.NewIdent(expMsgV)},
		})
	} else {
		ifBody = append(ifBody, &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{X: ast.NewIdent("slog"), Sel: ast.NewIdent("Warn")},
				Args: []ast.Expr{
					&ast.BasicLit{Kind: token.STRING, Value: `"flow.explain_error"`},
					&ast.BasicLit{Kind: token.STRING, Value: `"message"`},
					ast.NewIdent(expMsgV),
				},
			},
		})
	}
	stmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  errExpr,
			Op: token.NEQ,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{List: ifBody},
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}
