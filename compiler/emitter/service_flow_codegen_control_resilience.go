package emitter

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

type flowCapturedVar struct {
	typ   string
	isPtr bool
}

func renderFlowStepControlResilience(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	switch step.Action {
	case "flow.Try":
		if out, ok := renderFlowTryAST(st, step, indent, sfx, child); ok {
			return out, true
		}
		return renderFlowTryLegacy(st, step, indent, sfx, child), true
	case "flow.Retry":
		if out, ok := renderFlowRetryAST(st, step, indent, sfx, child); ok {
			return out, true
		}
		return renderFlowRetryLegacy(st, step, indent, sfx, child), true
	case "flow.Timeout":
		if out, ok := renderFlowTimeoutAST(st, step, indent, sfx, arg, child); ok {
			return out, true
		}
		return renderFlowTimeoutLegacy(st, step, indent, sfx, arg, child), true
	case "flow.Fallback":
		return renderFlowFallbackLegacy(st, step, indent, sfx, child), true
	}
	return "", false
}

func collectFlowBranchNewVars(st *flowRenderState, indent int, branches ...[]normalizer.FlowStep) map[string]flowCapturedVar {
	outerDeclared := make(map[string]bool, len(st.declared))
	for k, v := range st.declared {
		outerDeclared[k] = v
	}
	newVars := make(map[string]flowCapturedVar)
	for _, branch := range branches {
		probeState := cloneFlowState(st)
		probeState.returnErrOnly = true
		_ = renderFlowSteps(probeState, branch, indent+1)
		for varName := range probeState.declared {
			if outerDeclared[varName] {
				continue
			}
			goType := probeState.types[varName]
			if goType == "" {
				goType = "any"
			}
			newVars[varName] = flowCapturedVar{typ: goType, isPtr: probeState.pointers[varName]}
		}
	}
	return newVars
}

func renderFlowTryAST(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, child func(string) []normalizer.FlowStep) (string, bool) {
	doSteps := child("_do")
	catchSteps := child("_catch")
	if len(doSteps) == 0 {
		return "", true
	}

	retries := flowIntArg(step.Args, "retries", 0)
	backoffMs := flowIntArg(step.Args, "backoffMs", 0)

	newVars := collectFlowBranchNewVars(st, indent, doSteps, catchSteps)

	stmts := make([]ast.Stmt, 0, 12+len(newVars))
	newVarNames := make([]string, 0, len(newVars))
	for n := range newVars {
		newVarNames = append(newVarNames, n)
	}
	sort.Strings(newVarNames)
	for _, varName := range newVarNames {
		v := newVars[varName]
		typExpr, err := parseFlowExprSafe(v.typ)
		if err != nil {
			return "", false
		}
		stmts = append(stmts, &ast.DeclStmt{
			Decl: &ast.GenDecl{
				Tok: token.VAR,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names: []*ast.Ident{ast.NewIdent(varName)},
						Type:  typExpr,
					},
				},
			},
		})
		st.declared[varName] = true
		st.pointers[varName] = v.isPtr
		st.types[varName] = v.typ
	}

	tryRunV, tryErrV, tryMaxV, tryBackoffV := "_tryRun"+sfx, "_tryErr"+sfx, "_tryMax"+sfx, "_tryBackoff"+sfx
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(tryMaxV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{flowIntLit(retries)},
	})
	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(tryMaxV),
			Op: token.LSS,
			Y:  flowIntLit(0),
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(tryMaxV)},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{flowIntLit(0)},
				},
			},
		},
	})
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(tryBackoffV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{flowIntLit(backoffMs)},
	})

	tryState := cloneFlowState(st)
	tryState.returnErrOnly = true
	tryBodyStmts, err := parseFlowStmtList(renderFlowSteps(tryState, doSteps, 1) + "return nil\n")
	if err != nil {
		return "", false
	}
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(tryRunV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.FuncLit{
				Type: &ast.FuncType{
					Params: &ast.FieldList{},
					Results: &ast.FieldList{
						List: []*ast.Field{{Type: ast.NewIdent("error")}},
					},
				},
				Body: &ast.BlockStmt{List: tryBodyStmts},
			},
		},
	})
	stmts = append(stmts, &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{ast.NewIdent(tryErrV)},
					Type:  ast.NewIdent("error"),
				},
			},
		},
	})

	tryAttempt := ast.NewIdent("_tryAttempt")
	stmts = append(stmts, &ast.ForStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{tryAttempt},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{flowIntLit(0)},
		},
		Cond: &ast.BinaryExpr{
			X:  tryAttempt,
			Op: token.LEQ,
			Y:  ast.NewIdent(tryMaxV),
		},
		Post: &ast.IncDecStmt{
			X:   tryAttempt,
			Tok: token.INC,
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(tryErrV)},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: ast.NewIdent(tryRunV),
						},
					},
				},
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  ast.NewIdent(tryErrV),
						Op: token.EQL,
						Y:  ast.NewIdent("nil"),
					},
					Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}},
				},
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X:  tryAttempt,
							Op: token.LSS,
							Y:  ast.NewIdent(tryMaxV),
						},
						Op: token.LAND,
						Y: &ast.BinaryExpr{
							X:  ast.NewIdent(tryBackoffV),
							Op: token.GTR,
							Y:  flowIntLit(0),
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Sleep")},
									Args: []ast.Expr{
										&ast.BinaryExpr{
											X: &ast.CallExpr{
												Fun:  &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Duration")},
												Args: []ast.Expr{ast.NewIdent(tryBackoffV)},
											},
											Op: token.MUL,
											Y:  &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Millisecond")},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	errBody := []ast.Stmt{
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("_flowLastError")},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{ast.NewIdent(tryErrV)},
		},
	}
	if len(catchSteps) > 0 {
		catchBody, parseErr := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), catchSteps, 1))
		if parseErr != nil {
			return "", false
		}
		errBody = append(errBody, catchBody...)
	} else {
		returnBody, parseErr := parseFlowStmtList(errReturn(st, "", tryErrV))
		if parseErr != nil {
			return "", false
		}
		errBody = append(errBody, returnBody...)
	}
	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(tryErrV),
			Op: token.NEQ,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{List: errBody},
	})

	return renderFlowASTStmt(&ast.BlockStmt{List: stmts}, indent), true
}

func renderFlowRetryAST(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, child func(string) []normalizer.FlowStep) (string, bool) {
	doSteps := child("_do")
	catchSteps := child("_catch")
	if len(doSteps) == 0 {
		return "", true
	}

	attempts := flowIntArg(step.Args, "attempts", -1)
	if attempts < 0 {
		retries := flowIntArg(step.Args, "retries", -1)
		if retries >= 0 {
			attempts = retries + 1
		} else {
			attempts = 3
		}
	}
	if attempts <= 0 {
		attempts = 1
	}
	backoffMs := flowIntArg(step.Args, "backoffMs", 0)

	runV, errV, attemptsV, backoffV := "_retryRun"+sfx, "_retryErr"+sfx, "_retryAttempts"+sfx, "_retryBackoff"+sfx
	stmts := make([]ast.Stmt, 0, 9)
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(attemptsV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{flowIntLit(attempts)},
	})
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(backoffV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{flowIntLit(backoffMs)},
	})

	retryState := cloneFlowState(st)
	retryState.returnErrOnly = true
	retryBodyStmts, err := parseFlowStmtList(renderFlowSteps(retryState, doSteps, 1) + "return nil\n")
	if err != nil {
		return "", false
	}
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(runV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.FuncLit{
				Type: &ast.FuncType{
					Params: &ast.FieldList{},
					Results: &ast.FieldList{
						List: []*ast.Field{{Type: ast.NewIdent("error")}},
					},
				},
				Body: &ast.BlockStmt{List: retryBodyStmts},
			},
		},
	})
	stmts = append(stmts, &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{ast.NewIdent(errV)},
					Type:  ast.NewIdent("error"),
				},
			},
		},
	})

	tryAttempt := ast.NewIdent("_tryAttempt")
	stmts = append(stmts, &ast.ForStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{tryAttempt},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{flowIntLit(0)},
		},
		Cond: &ast.BinaryExpr{
			X:  tryAttempt,
			Op: token.LSS,
			Y:  ast.NewIdent(attemptsV),
		},
		Post: &ast.IncDecStmt{X: tryAttempt, Tok: token.INC},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(errV)},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: ast.NewIdent(runV),
						},
					},
				},
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  ast.NewIdent(errV),
						Op: token.EQL,
						Y:  ast.NewIdent("nil"),
					},
					Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}},
				},
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X: &ast.BinaryExpr{
								X:  tryAttempt,
								Op: token.ADD,
								Y:  flowIntLit(1),
							},
							Op: token.LSS,
							Y:  ast.NewIdent(attemptsV),
						},
						Op: token.LAND,
						Y: &ast.BinaryExpr{
							X:  ast.NewIdent(backoffV),
							Op: token.GTR,
							Y:  flowIntLit(0),
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Sleep")},
									Args: []ast.Expr{
										&ast.BinaryExpr{
											X: &ast.CallExpr{
												Fun:  &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Duration")},
												Args: []ast.Expr{ast.NewIdent(backoffV)},
											},
											Op: token.MUL,
											Y:  &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Millisecond")},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	errBody := []ast.Stmt{
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("_flowLastError")},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{ast.NewIdent(errV)},
		},
	}
	if len(catchSteps) > 0 {
		catchBody, parseErr := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), catchSteps, 1))
		if parseErr != nil {
			return "", false
		}
		errBody = append(errBody, catchBody...)
	} else {
		returnBody, parseErr := parseFlowStmtList(errReturn(st, "", errV))
		if parseErr != nil {
			return "", false
		}
		errBody = append(errBody, returnBody...)
	}
	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(errV),
			Op: token.NEQ,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{List: errBody},
	})

	return renderFlowASTStmt(&ast.BlockStmt{List: stmts}, indent), true
}

func renderFlowTimeoutAST(st *flowRenderState, _ normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	duration := arg("duration")
	doSteps := child("_do")
	onTimeout := child("_onTimeout")
	if duration == "" || len(doSteps) == 0 {
		return "", true
	}
	durationExpr, err := parseFlowExprSafe(duration)
	if err != nil {
		return "", false
	}

	toCtxV, toCancelV, toRunV, toErrV := "_toCtx"+sfx, "_toCancel"+sfx, "_toRun"+sfx, "_toErr"+sfx
	stmts := make([]ast.Stmt, 0, 6)
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(toCtxV), ast.NewIdent(toCancelV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{X: ast.NewIdent("context"), Sel: ast.NewIdent("WithTimeout")},
				Args: []ast.Expr{
					ast.NewIdent("ctx"),
					durationExpr,
				},
			},
		},
	})
	stmts = append(stmts, &ast.DeferStmt{
		Call: &ast.CallExpr{Fun: ast.NewIdent(toCancelV)},
	})

	toState := cloneFlowState(st)
	toState.returnErrOnly = true
	doBodyStmts, parseErr := parseFlowStmtList(renderFlowSteps(toState, doSteps, 1) + "return nil\n")
	if parseErr != nil {
		return "", false
	}
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(toRunV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.FuncLit{
				Type: &ast.FuncType{
					Params: &ast.FieldList{
						List: []*ast.Field{
							{
								Names: []*ast.Ident{ast.NewIdent("ctx")},
								Type:  &ast.SelectorExpr{X: ast.NewIdent("context"), Sel: ast.NewIdent("Context")},
							},
						},
					},
					Results: &ast.FieldList{
						List: []*ast.Field{{Type: ast.NewIdent("error")}},
					},
				},
				Body: &ast.BlockStmt{List: doBodyStmts},
			},
		},
	})
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(toErrV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun:  ast.NewIdent(toRunV),
				Args: []ast.Expr{ast.NewIdent(toCtxV)},
			},
		},
	})

	timeoutBranch := []ast.Stmt{}
	if len(onTimeout) > 0 {
		timeoutBranch, parseErr = parseFlowStmtList(renderFlowSteps(cloneFlowState(st), onTimeout, 1))
		if parseErr != nil {
			return "", false
		}
	} else {
		timeoutBranch, parseErr = parseFlowStmtList(errReturn(st, "", "errors.New(http.StatusGatewayTimeout, \"TIMEOUT\", \"flow step timed out\")"))
		if parseErr != nil {
			return "", false
		}
	}
	elseBranch, parseErr := parseFlowStmtList(errReturn(st, "", toErrV))
	if parseErr != nil {
		return "", false
	}
	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(toErrV),
			Op: token.NEQ,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("_flowLastError")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent(toErrV)},
				},
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.CallExpr{
							Fun: &ast.SelectorExpr{X: ast.NewIdent(toCtxV), Sel: ast.NewIdent("Err")},
						},
						Op: token.EQL,
						Y:  &ast.SelectorExpr{X: ast.NewIdent("context"), Sel: ast.NewIdent("DeadlineExceeded")},
					},
					Body: &ast.BlockStmt{List: timeoutBranch},
					Else: &ast.BlockStmt{List: elseBranch},
				},
			},
		},
	})

	return renderFlowASTStmt(&ast.BlockStmt{List: stmts}, indent), true
}

func renderFlowTryLegacy(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, child func(string) []normalizer.FlowStep) string {
	pad := strings.Repeat("\t", indent)
	doSteps := child("_do")
	catchSteps := child("_catch")
	if len(doSteps) == 0 {
		return ""
	}
	retries := flowIntArg(step.Args, "retries", 0)
	backoffMs := flowIntArg(step.Args, "backoffMs", 0)

	newVars := collectFlowBranchNewVars(st, indent, doSteps, catchSteps)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	newVarNames := make([]string, 0, len(newVars))
	for n := range newVars {
		newVarNames = append(newVarNames, n)
	}
	sort.Strings(newVarNames)
	for _, varName := range newVarNames {
		v := newVars[varName]
		b.WriteString(fmt.Sprintf("%s\tvar %s %s\n", pad, varName, v.typ))
		st.declared[varName] = true
		st.pointers[varName] = v.isPtr
		st.types[varName] = v.typ
	}

	tryRunV, tryErrV, tryMaxV, tryBackoffV := "_tryRun"+sfx, "_tryErr"+sfx, "_tryMax"+sfx, "_tryBackoff"+sfx
	b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, tryMaxV, retries))
	b.WriteString(fmt.Sprintf("%s\tif %s < 0 { %s = 0 }\n", pad, tryMaxV, tryMaxV))
	b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, tryBackoffV, backoffMs))
	b.WriteString(fmt.Sprintf("%s\t%s := func() error {\n", pad, tryRunV))
	tryState := cloneFlowState(st)
	tryState.returnErrOnly = true
	b.WriteString(renderFlowSteps(tryState, doSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, tryErrV))
	b.WriteString(fmt.Sprintf("%s\tfor _tryAttempt := 0; _tryAttempt <= %s; _tryAttempt++ {\n", pad, tryMaxV))
	b.WriteString(fmt.Sprintf("%s\t\t%s = %s()\n", pad, tryErrV, tryRunV))
	b.WriteString(fmt.Sprintf("%s\t\tif %s == nil {\n", pad, tryErrV))
	b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif _tryAttempt < %s && %s > 0 {\n", pad, tryMaxV, tryBackoffV))
	b.WriteString(fmt.Sprintf("%s\t\t\ttime.Sleep(time.Duration(%s) * time.Millisecond)\n", pad, tryBackoffV))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, tryErrV))
	b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, tryErrV))
	if len(catchSteps) > 0 {
		b.WriteString(renderFlowSteps(cloneFlowState(st), catchSteps, indent+2))
	} else {
		b.WriteString(errReturn(st, pad+"\t\t", tryErrV))
	}
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowRetryLegacy(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, child func(string) []normalizer.FlowStep) string {
	pad := strings.Repeat("\t", indent)
	doSteps := child("_do")
	catchSteps := child("_catch")
	if len(doSteps) == 0 {
		return ""
	}
	attempts := flowIntArg(step.Args, "attempts", -1)
	if attempts < 0 {
		retries := flowIntArg(step.Args, "retries", -1)
		if retries >= 0 {
			attempts = retries + 1
		} else {
			attempts = 3
		}
	}
	if attempts <= 0 {
		attempts = 1
	}
	backoffMs := flowIntArg(step.Args, "backoffMs", 0)

	runV, errV, attemptsV, backoffV := "_retryRun"+sfx, "_retryErr"+sfx, "_retryAttempts"+sfx, "_retryBackoff"+sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, attemptsV, attempts))
	b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, backoffV, backoffMs))
	b.WriteString(fmt.Sprintf("%s\t%s := func() error {\n", pad, runV))
	retryState := cloneFlowState(st)
	retryState.returnErrOnly = true
	b.WriteString(renderFlowSteps(retryState, doSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, errV))
	b.WriteString(fmt.Sprintf("%s\tfor _tryAttempt := 0; _tryAttempt < %s; _tryAttempt++ {\n", pad, attemptsV))
	b.WriteString(fmt.Sprintf("%s\t\t%s = %s()\n", pad, errV, runV))
	b.WriteString(fmt.Sprintf("%s\t\tif %s == nil {\n", pad, errV))
	b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif _tryAttempt+1 < %s && %s > 0 {\n", pad, attemptsV, backoffV))
	b.WriteString(fmt.Sprintf("%s\t\t\ttime.Sleep(time.Duration(%s) * time.Millisecond)\n", pad, backoffV))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, errV))
	b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, errV))
	if len(catchSteps) > 0 {
		b.WriteString(renderFlowSteps(cloneFlowState(st), catchSteps, indent+2))
	} else {
		b.WriteString(errReturn(st, pad+"\t\t", errV))
	}
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowFallbackLegacy(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, child func(string) []normalizer.FlowStep) string {
	pad := strings.Repeat("\t", indent)
	mainSteps := child("_do")
	fallbackSteps := child("_fallback")
	if len(mainSteps) == 0 || len(fallbackSteps) == 0 {
		return ""
	}
	runV, errV := "_fbRun"+sfx, "_fbErr"+sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := func() error {\n", pad, runV))
	fbState := cloneFlowState(st)
	fbState.returnErrOnly = true
	b.WriteString(renderFlowSteps(fbState, mainSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := %s()\n", pad, errV, runV))
	b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, errV))
	b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, errV))
	b.WriteString(renderFlowSteps(cloneFlowState(st), fallbackSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowTimeoutLegacy(st *flowRenderState, _ normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	pad := strings.Repeat("\t", indent)
	duration := arg("duration")
	doSteps := child("_do")
	onTimeout := child("_onTimeout")
	if duration == "" || len(doSteps) == 0 {
		return ""
	}
	toCtxV, toCancelV, toRunV, toErrV := "_toCtx"+sfx, "_toCancel"+sfx, "_toRun"+sfx, "_toErr"+sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s, %s := context.WithTimeout(ctx, %s)\n", pad, toCtxV, toCancelV, duration))
	b.WriteString(fmt.Sprintf("%s\tdefer %s()\n", pad, toCancelV))
	b.WriteString(fmt.Sprintf("%s\t%s := func(ctx context.Context) error {\n", pad, toRunV))
	toState := cloneFlowState(st)
	toState.returnErrOnly = true
	b.WriteString(renderFlowSteps(toState, doSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := %s(%s)\n", pad, toErrV, toRunV, toCtxV))
	b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, toErrV))
	b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, toErrV))
	b.WriteString(fmt.Sprintf("%s\t\tif %s.Err() == context.DeadlineExceeded {\n", pad, toCtxV))
	if len(onTimeout) > 0 {
		b.WriteString(renderFlowSteps(cloneFlowState(st), onTimeout, indent+3))
	} else {
		b.WriteString(errReturn(st, pad+"\t\t\t", "errors.New(http.StatusGatewayTimeout, \"TIMEOUT\", \"flow step timed out\")"))
	}
	b.WriteString(fmt.Sprintf("%s\t\t} else {\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t\t", toErrV))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}
