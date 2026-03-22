package emitter

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
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
		if out, ok := renderFlowFallbackAST(st, step, indent, sfx, child); ok {
			return out, true
		}
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
				goType = inferFlowCapturedVarType(varName, branch)
			}
			if goType == "" {
				goType = "any"
			}
			newVars[varName] = flowCapturedVar{typ: goType, isPtr: probeState.pointers[varName]}
		}
	}
	return newVars
}

func inferFlowCapturedVarType(varName string, steps []normalizer.FlowStep) string {
	for _, step := range steps {
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) == varName {
			switch step.Action {
			case "fs.TempDir", "fs.ReadFile", "str.Concat", "flow.ExplainError", "model.Resolve", "config.Get", "json.Stringify", "json.Marshal", "template.Render":
				return "string"
			case "repo.Exists":
				return "bool"
			case "repo.Count":
				return "int"
			case "openai.Chat":
				if len(parseOpenAIToolNames(step)) > 0 {
					return "struct{ Content string; FinishReason string; ToolCalls int; PromptTokens int; CompletionTokens int; TotalTokens int }"
				}
				return "string"
			case "openai.Embed":
				return "[]float64"
			case "cue.WriteProjectFiles", "cue.ValidateProject", "plan.BuildAutomata", "plan.BuildMicroPlan":
				return "map[string]any"
			case "cue.EmitProject":
				return "map[string]string"
			case "json.Parse":
				if into, _ := step.Args["into"].(string); strings.TrimSpace(into) != "" {
					return strings.TrimSpace(into)
				}
			case "map.Get":
				if into, _ := step.Args["into"].(string); strings.TrimSpace(into) != "" {
					return strings.TrimSpace(into)
				}
				return "any"
			}
		}
		for _, key := range []string{"_do", "_catch", "_then", "_else", "_fallback", "_onTimeout", "_default", "_onMissing"} {
			if nested, ok := step.Args[key].([]normalizer.FlowStep); ok {
				if typ := inferFlowCapturedVarType(varName, nested); typ != "" {
					return typ
				}
			}
		}
		if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
			for _, nested := range cases {
				if typ := inferFlowCapturedVarType(varName, nested); typ != "" {
					return typ
				}
			}
		}
		if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok {
			for _, nested := range branches {
				if typ := inferFlowCapturedVarType(varName, nested); typ != "" {
					return typ
				}
			}
		}
	}
	return ""
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

func renderFlowFallbackAST(st *flowRenderState, _ normalizer.FlowStep, indent int, sfx string, child func(string) []normalizer.FlowStep) (string, bool) {
	mainSteps := child("_do")
	fallbackSteps := child("_fallback")
	if len(mainSteps) == 0 || len(fallbackSteps) == 0 {
		return "", true
	}

	runV, errV := "_fbRun"+sfx, "_fbErr"+sfx
	stmts := make([]ast.Stmt, 0, 3)

	runState := cloneFlowState(st)
	runState.returnErrOnly = true
	runBody, err := parseFlowStmtList(renderFlowSteps(runState, mainSteps, 1) + "return nil\n")
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
				Body: &ast.BlockStmt{List: runBody},
			},
		},
	})
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(errV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: ast.NewIdent(runV),
			},
		},
	})

	fallbackBody, parseErr := parseFlowStmtList(renderFlowSteps(cloneFlowState(st), fallbackSteps, 1))
	if parseErr != nil {
		return "", false
	}
	ifBody := []ast.Stmt{
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("_flowLastError")},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{ast.NewIdent(errV)},
		},
	}
	ifBody = append(ifBody, fallbackBody...)

	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(errV),
			Op: token.NEQ,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{List: ifBody},
	})

	return renderFlowASTStmt(&ast.BlockStmt{List: stmts}, indent), true
}
