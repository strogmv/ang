package emitter

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowStepControlFlow(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "flow.If":
		typed, err := decodeCurrentActionAs[flowir.FlowIf](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, "flow.If", err.Error()), true
		}
		typedArg := func(name string) string {
			if name == "condition" {
				return normalizeFlowExpr(typed.Condition.Source)
			}
			return arg(name)
		}
		typedChild := child
		if out, ok := renderFlowIfAST(st, indent, typedArg, typedChild); ok {
			return out, true
		}
		return renderFlowIfLegacy(st, pad, indent, typedArg, typedChild), true

	case "flow.For":
		typed, err := decodeCurrentActionAs[flowir.FlowFor](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		typedArg := func(n string) string {
			if n == "each" {
				return normalizeFlowExpr(typed.Each.Source)
			}
			if n == "as" {
				return typed.As
			}
			return arg(n)
		}
		typedChild := child
		if out, ok := renderFlowForAST(st, indent, typedArg, typedChild); ok {
			return out, true
		}
		return renderFlowForLegacy(st, pad, indent, typedArg, typedChild), true

	case "flow.Block", "tx.Block":
		_, err := decodeCurrentActionAs[flowir.FlowBlock](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		typedChild := child
		if out, ok := renderFlowBlockAST(st, indent, typedChild); ok {
			return out, true
		}
		return renderFlowBlockLegacy(st, indent, typedChild), true

	case "flow.Switch":
		typed, err := decodeCurrentActionAs[flowir.FlowSwitch](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		typedArg := func(n string) string {
			if n == "value" {
				return normalizeFlowExpr(typed.Value.Source)
			}
			if n == "match" {
				return typed.Match
			}
			return arg(n)
		}
		if out, ok := renderFlowSwitchAST(st, nil, indent, typedArg, nil); ok {
			return out, true
		}
		return renderFlowSwitchLegacy(st, nil, pad, indent, typedArg, nil), true

	case "flow.While":
		typed, err := decodeCurrentActionAs[flowir.FlowWhile](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		typedArg := func(n string) string {
			if n == "condition" {
				return normalizeFlowExpr(typed.Condition.Source)
			}
			return arg(n)
		}
		typedChild := child
		if out, ok := renderFlowWhileAST(st, indent, typedArg, typedChild); ok {
			return out, true
		}
		return renderFlowWhileLegacy(st, pad, indent, typedArg, typedChild), true

	case "flow.Call":
		typed, err := decodeCurrentActionAs[flowir.FlowCall](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowCall(st, typed, pad), true

	case "flow.Checkpoint":
		return "", true

	case "flow.Resume":
		return "", true

	case "flow.RecordEvent":
		typed, err := decodeCurrentActionAs[flowir.FlowRecordEvent](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowRecordEvent(st, typed, indent, sfx), true

	case "flow.History.Get":
		typed, err := decodeCurrentActionAs[flowir.FlowHistoryGet](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowHistoryGet(st, typed, indent, sfx), true

	case "flow.Replay":
		typed, err := decodeCurrentActionAs[flowir.FlowReplay](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowReplay(st, typed, indent, sfx), true

	case "flow.Validate":
		return "", true

	case "flow.Catch":
		return "", true

	case "flow.Defer":
		return "", true

	case "flow.SuggestNext":
		return "", true

	case "flow.ExplainError":
		return "", true

	case "flow.Parallel":
		_, err := decodeCurrentActionAs[flowir.FlowParallel](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowParallel(st, nil, indent, sfx), true

	case "flow.Join":
		_, err := decodeCurrentActionAs[flowir.FlowJoin](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowJoin(st, nil, indent, sfx), true

	case "flow.Race":
		_, err := decodeCurrentActionAs[flowir.FlowRace](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowRace(st, nil, indent, sfx), true

	case "flow.Delay":
		typed, err := decodeCurrentActionAs[flowir.FlowDelay](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowDelay(st, typed, indent, sfx), true

	case "flow.Schedule":
		typed, err := decodeCurrentActionAs[flowir.FlowSchedule](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowSchedule(st, typed, indent, sfx), true

	case "flow.Cron":
		typed, err := decodeCurrentActionAs[flowir.FlowCron](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowCron(st, typed, indent, sfx), true

	case "flow.Tag":
		typed, err := decodeCurrentActionAs[flowir.FlowTag](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderFlowTag(st, typed, indent), true

	case "flow.Return":
		typed, err := decodeCurrentActionAs[flowir.FlowReturn](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		// Early return from the flow with the current response value.
		// Optional: set a field before returning:
		//   { action: "flow.Return", set: "resp.Status", value: `"ok"` }
		setField := normalizeFlowExpr(typed.Set.Source)
		setValue := normalizeFlowExpr(typed.Value.Source)
		var b strings.Builder
		if setField != "" && setValue != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, setField, setValue))
		}
		b.WriteString(returnSuccess(st, pad))
		return b.String(), true
	}

	return "", false
}

func renderFlowTag(st *flowRenderState, action flowir.FlowTag, indent int) string {
	pad := strings.Repeat("\t", indent)
	name := normalizeFlowExpr(action.Name.Source)
	value := normalizeFlowExpr(action.Value.Source)
	if name == "" {
		return renderInvalidFlowStepConfig(st, pad, "flow.Tag", "flow.Tag requires name")
	}

	if value != "" {
		return fmt.Sprintf("%sslog.Info(\"flow.tag\", \"name\", %s, \"value\", %s)\n", pad, name, value)
	}
	return fmt.Sprintf("%sslog.Info(\"flow.tag\", \"name\", %s)\n", pad, name)
}

func renderFlowCall(st *flowRenderState, action flowir.FlowCall, pad string) string {
	opRaw := action.Operation
	if opRaw == "" {
		return renderInvalidFlowStepConfig(st, pad, "flow.Call", "flow.Call requires op")
	}
	output, ignoreErr, ignoreErrReason := action.Output, action.IgnoreError, action.IgnoreErrReason

	serviceName := ""
	methodName := opRaw
	if parts := strings.SplitN(opRaw, ".", 2); len(parts) == 2 {
		serviceName = strings.TrimSpace(parts[0])
		methodName = strings.TrimSpace(parts[1])
	}
	if methodName == "" {
		return renderInvalidFlowStepConfig(st, pad, "flow.Call", "flow.Call requires valid op method name")
	}
	methodExport := ExportName(methodName)

	reqExpr := fmt.Sprintf("port.%sRequest{}", methodExport)
	argMap := action.Arguments
	if len(argMap) > 0 {
		keys := make([]string, 0, len(argMap))
		for k := range argMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fields := make([]string, 0, len(keys))
		for _, k := range keys {
			expr := normalizeFlowExpr(argMap[k].Source)
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
		if ignoreErrReason == "" {
			emitFlowWarning(st, "FLOW_IGNORE_ERR", "warn", "flow.Call ignores returned error explicitly", "Document intent with ignoreErrReason and use only for deliberate fire-and-forget behavior")
		}
		comment := fmt.Sprintf("%s// explicit ignoreErr=true", pad)
		if ignoreErrReason != "" {
			comment = fmt.Sprintf("%s// explicit ignoreErr=true: %s", pad, ignoreErrReason)
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "port." + methodExport + "Response"
			return fmt.Sprintf("%s\n%s%s %s %s\n", comment, pad, output+", _", assign, callStr)
		}
		return fmt.Sprintf("%s\n%s_, _ = %s\n", comment, pad, callStr)
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
	thenBody, err := parseFlowStmtList(renderFlowChildSteps(cloneFlowState(st), child, "_then", 1))
	if err != nil {
		return "", false
	}
	stmt := &ast.IfStmt{
		Cond: condExpr,
		Body: &ast.BlockStmt{List: thenBody},
	}
	var typedElse []flowir.TypedStep
	var rawElse []normalizer.FlowStep
	if st.currentTyped != nil {
		typedElse = st.currentTyped.Children["_else"]
	} else {
		rawElse = child("_else")
	}
	elseCount := len(typedElse)
	if st.currentTyped == nil {
		elseCount = len(rawElse)
	}
	if elseCount > 0 {
		// If else contains a single flow.If step, render as "else if" (ast.IfStmt as Else)
		elseAction := ""
		if len(typedElse) == 1 {
			elseAction = typedElse[0].Name
		} else if len(rawElse) == 1 {
			elseAction = rawElse[0].Action
		}
		if elseCount == 1 && elseAction == "flow.If" {
			var typedNested []flowir.TypedStep
			if st.currentTyped != nil {
				typedNested = typedElse
			} else {
				typedNested, _ = flowir.DecodeSteps(rawElse)
			}
			if len(typedNested) != 1 {
				return "", false
			}
			nestedStep := typedNested[0]
			nestedAction, ok := nestedStep.Action.(flowir.FlowIf)
			if !ok {
				return "", false
			}
			nestedArg := func(key string) string {
				if key == "condition" {
					return normalizeFlowExpr(nestedAction.Condition.Source)
				}
				return ""
			}
			nestedChild := func(key string) []normalizer.FlowStep {
				return nil
			}
			nestedState := cloneFlowState(st)
			nestedState.currentTyped = &nestedStep
			nestedStr, ok := renderFlowIfAST(nestedState, 1, nestedArg, nestedChild)
			if ok && nestedStr != "" {
				nestedStmts, parseErr := parseFlowStmtList(nestedStr)
				if parseErr == nil && len(nestedStmts) == 1 {
					if nestedIf, ok2 := nestedStmts[0].(*ast.IfStmt); ok2 {
						stmt.Else = nestedIf
					}
				}
			}
		}
		if stmt.Else == nil {
			elseBody, parseErr := parseFlowStmtList(renderFlowChildSteps(cloneFlowState(st), child, "_else", 1))
			if parseErr != nil {
				return "", false
			}
			stmt.Else = &ast.BlockStmt{List: elseBody}
		}
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func renderFlowBlockAST(st *flowRenderState, indent int, child func(string) []normalizer.FlowStep) (string, bool) {
	body, err := parseFlowStmtList(renderFlowChildSteps(st, child, "_do", 1))
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
	body, err := parseFlowStmtList(renderFlowChildSteps(cloneFlowState(st), child, "_do", 1))
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

func renderFlowSwitchAST(st *flowRenderState, cases map[string][]normalizer.FlowStep, indent int, arg func(string) string, defaultSteps []normalizer.FlowStep) (string, bool) {
	value := arg("value")
	if value == "" {
		return "", true
	}
	matchMode := strings.ToLower(strings.TrimSpace(arg("match")))
	if matchMode != "" && matchMode != "exact" {
		return "", false
	}
	valueExpr, err := parseFlowExprSafe(value)
	if err != nil {
		return "", false
	}
	keys := flowBranchNames(st, cases)
	clauses := make([]ast.Stmt, 0, len(keys)+1)
	for _, k := range keys {
		body, parseErr := parseFlowStmtList(renderFlowBranchSteps(cloneFlowState(st), k, cases[k], 1))
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
	if flowNestedStepCount(st, "_default", defaultSteps) > 0 {
		body, parseErr := parseFlowStmtList(renderFlowNestedSteps(cloneFlowState(st), "_default", defaultSteps, 1))
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
	body, err := parseFlowStmtList(renderFlowChildSteps(cloneFlowState(st), child, "_do", 1))
	if err != nil {
		return "", false
	}
	stmt := &ast.ForStmt{
		Cond: condExpr,
		Body: &ast.BlockStmt{List: body},
	}
	return renderFlowASTStmts([]ast.Stmt{stmt}, indent), true
}

func renderFlowCheckpointAST(indent int, action flowir.FlowCheckpoint) (string, bool) {
	name := action.Name
	if name == "" {
		return "", true
	}
	data := normalizeFlowExpr(action.Data.Source)
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
	if flowChildStepCount(st, child, "_onMissing") > 0 {
		parsed, err := parseFlowStmtList(renderFlowChildSteps(cloneFlowState(st), child, "_onMissing", 1))
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

func renderFlowCatchAST(st *flowRenderState, catchSteps []normalizer.FlowStep, indent int) (string, bool) {
	if flowNestedStepCount(st, "_do", catchSteps) == 0 {
		return "", true
	}
	catchBody, err := parseFlowStmtList(renderFlowNestedSteps(cloneFlowState(st), "_do", catchSteps, 1))
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

func renderFlowDeferAST(st *flowRenderState, deferSteps []normalizer.FlowStep, indent int) (string, bool) {
	if flowNestedStepCount(st, "_do", deferSteps) == 0 {
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
	body, err := parseFlowStmtList(renderFlowNestedSteps(deferState, "_do", deferSteps, 1))
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

func renderTypedFlowDefer(st *flowRenderState, step flowir.TypedStep, pad string, indent int) string {
	deferSteps := step.Children["_do"]
	if len(deferSteps) == 0 {
		return ""
	}
	predecl := flowTypedDeferPredeclaredStringVars(st, deferSteps)
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
	b.WriteString(renderTypedFlowSteps(deferState, deferSteps, indent+1))
	b.WriteString(fmt.Sprintf("%s}()\n", pad))
	return b.String()
}

func flowDeferPredeclaredStringVars(st *flowRenderState, steps []normalizer.FlowStep) []string {
	if st.currentTyped != nil {
		return flowTypedDeferPredeclaredStringVars(st, st.currentTyped.Children["_do"])
	}
	typed, _ := flowir.DecodeSteps(steps)
	return flowTypedDeferPredeclaredStringVars(st, typed)
}

func flowTypedDeferPredeclaredStringVars(st *flowRenderState, steps []flowir.TypedStep) []string {
	set := map[string]struct{}{}
	var walk func([]flowir.TypedStep)
	walk = func(items []flowir.TypedStep) {
		for _, s := range items {
			if action, ok := s.Action.(flowir.FSRemove); ok {
				if ident, ok := flowSimpleIdent(normalizeFlowExpr(action.Path.Source)); ok && !st.declared[ident] {
					set[ident] = struct{}{}
				}
			}
			for _, nested := range s.Children {
				walk(nested)
			}
			for _, nested := range s.Branches {
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

func renderFlowSuggestNextAST(st *flowRenderState, action flowir.FlowSuggestNext, indent int) (string, bool) {
	output, options := action.Output, action.Options
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

func renderFlowValidateAST(st *flowRenderState, action flowir.FlowValidate, indent int) (string, bool) {
	cond := normalizeFlowExpr(action.Condition.Source)
	if cond == "" {
		return "", true
	}
	condExpr, err := parseFlowExprSafe(cond)
	if err != nil {
		return "", false
	}
	message := action.Message
	if hint := action.Hint; hint != "" {
		message = message + " (hint: " + hint + ")"
	}
	code, status := action.Code, normalizeFlowExpr(action.Status.Source)
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

func renderFlowExplainErrorAST(st *flowRenderState, action flowir.FlowExplainError, indent int, sfx string) (string, bool) {
	errExprRaw := normalizeFlowExpr(action.Error.Source)
	errExpr, err := parseFlowExprSafe(errExprRaw)
	if err != nil {
		return "", false
	}
	output, message, hint := action.Output, action.Message, action.Hint
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
