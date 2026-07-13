package emitter

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

type flowCapturedVar struct {
	typ   string
	isPtr bool
}

// collectFlowBranchNewVars is retained only for legacy renderer compatibility.
// Typed renderers use collectTypedFlowBranchNewVars directly.
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

func renderTypedStepControlResilience(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	switch step.Name {
	case "flow.Try":
		typed, err := typedActionAs[flowir.FlowTry](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, strings.Repeat("\t", indent), step.Name, err.Error()), true
		}
		if out, ok := renderFlowTryAST(st, step, typed, indent, sfx); ok {
			return out, true
		}
		return renderFlowTryLegacy(st, typed, indent, sfx), true
	case "flow.Retry":
		typed, err := typedActionAs[flowir.FlowRetry](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, strings.Repeat("\t", indent), step.Name, err.Error()), true
		}
		if out, ok := renderFlowRetryAST(st, step, typed, indent, sfx); ok {
			return out, true
		}
		return renderFlowRetryLegacy(st, typed, indent, sfx), true
	case "flow.Timeout":
		typed, err := typedActionAs[flowir.FlowTimeout](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, strings.Repeat("\t", indent), step.Name, err.Error()), true
		}
		if out, ok := renderFlowTimeoutAST(st, step, typed, indent, sfx); ok {
			return out, true
		}
		return renderFlowTimeoutLegacy(st, typed, indent, sfx), true
	case "flow.Fallback":
		typed, err := typedActionAs[flowir.FlowFallback](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, strings.Repeat("\t", indent), step.Name, err.Error()), true
		}
		if out, ok := renderFlowFallbackAST(st, step, typed, indent, sfx); ok {
			return out, true
		}
		return renderFlowFallbackLegacy(st, typed, indent, sfx), true
	}
	return "", false
}

func collectTypedFlowBranchNewVars(st *flowRenderState, indent int, branches ...[]flowir.TypedStep) map[string]flowCapturedVar {
	outerDeclared := make(map[string]bool, len(st.declared))
	for key, declared := range st.declared {
		outerDeclared[key] = declared
	}
	newVars := make(map[string]flowCapturedVar)
	for _, branch := range branches {
		probeState := cloneFlowState(st)
		probeState.returnErrOnly = true
		_ = renderTypedFlowSteps(probeState, branch, indent+1)
		for varName := range probeState.declared {
			if outerDeclared[varName] {
				continue
			}
			goType := probeState.types[varName]
			if goType == "" {
				goType = inferTypedCapturedVarType(varName, branch)
			}
			if goType == "" {
				goType = "any"
			}
			newVars[varName] = flowCapturedVar{typ: goType, isPtr: probeState.pointers[varName]}
		}
	}
	return newVars
}

// inferFlowCapturedVarType is a legacy test/renderer bridge. New production
// code must use inferTypedCapturedVarType after the single Flow IR decode.
func inferFlowCapturedVarType(varName string, steps []normalizer.FlowStep) string {
	typed, _ := flowir.DecodeSteps(steps)
	return inferTypedCapturedVarType(varName, typed)
}

func inferTypedCapturedVarType(varName string, steps []flowir.TypedStep) string {
	for _, step := range steps {
		if chat, ok := step.Action.(flowir.OpenAIChat); ok && chat.Output == varName && len(chat.Tools) > 0 {
			return "struct{ Content string; FinishReason string; ToolCalls int; PromptTokens int; CompletionTokens int; TotalTokens int }"
		}
		if emit, ok := step.Action.(flowir.CueEmitProject); ok && emit.Output == varName {
			return "map[string]string"
		}
		if step.Action != nil {
			for _, variable := range step.Action.DeclaredVariables() {
				if variable.Name == varName {
					if typ := flowIRTypeRefGoType(variable.Type); typ != "" {
						return typ
					}
				}
			}
		}
		for _, children := range step.Children {
			if typ := inferTypedCapturedVarType(varName, children); typ != "" {
				return typ
			}
		}
		for _, branch := range step.Branches {
			if typ := inferTypedCapturedVarType(varName, branch); typ != "" {
				return typ
			}
		}
	}
	return ""
}

func flowIRTypeRefGoType(typ flowir.TypeRef) string {
	switch typ.Kind {
	case flowir.TypeString:
		return "string"
	case flowir.TypeBool:
		return "bool"
	case flowir.TypeInt:
		return "int"
	case flowir.TypeFloat:
		return "float64"
	case flowir.TypeBytes:
		return "[]byte"
	case flowir.TypeTime:
		return "time.Time"
	case flowir.TypeDuration:
		return "time.Duration"
	case flowir.TypeMap:
		return "map[string]any"
	case flowir.TypeEntity:
		return "domain." + typ.Name
	case flowir.TypeDTO:
		return "port." + typ.Name
	case flowir.TypeList:
		if typ.Elem != nil {
			return "[]" + flowIRTypeRefGoType(*typ.Elem)
		}
		return "[]any"
	case flowir.TypePointer:
		if typ.Elem != nil {
			return "*" + flowIRTypeRefGoType(*typ.Elem)
		}
	case flowir.TypeUnknown:
		if typ.Name != "" {
			return typ.Name
		}
	}
	return ""
}

func renderFlowTryAST(st *flowRenderState, step flowir.TypedStep, action flowir.FlowTry, indent int, sfx string) (string, bool) {
	doSteps, catchSteps := step.Children["_do"], step.Children["_catch"]
	if len(doSteps) == 0 {
		return "", true
	}

	retries, backoffMs := action.Retries, action.BackoffMS

	newVars := collectTypedFlowBranchNewVars(st, indent, doSteps, catchSteps)

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
	tryBodyStmts, err := parseFlowStmtList(renderTypedFlowSteps(tryState, doSteps, 1) + "return nil\n")
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
		catchBody, parseErr := parseFlowStmtList(renderTypedFlowSteps(cloneFlowState(st), catchSteps, 1))
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

func renderFlowRetryAST(st *flowRenderState, step flowir.TypedStep, action flowir.FlowRetry, indent int, sfx string) (string, bool) {
	doSteps, catchSteps := step.Children["_do"], step.Children["_catch"]
	if len(doSteps) == 0 {
		return "", true
	}

	attempts, backoffMs := action.Attempts, action.BackoffMS

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
	retryBodyStmts, err := parseFlowStmtList(renderTypedFlowSteps(retryState, doSteps, 1) + "return nil\n")
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
		catchBody, parseErr := parseFlowStmtList(renderTypedFlowSteps(cloneFlowState(st), catchSteps, 1))
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

func renderFlowTimeoutAST(st *flowRenderState, step flowir.TypedStep, action flowir.FlowTimeout, indent int, sfx string) (string, bool) {
	duration := normalizeFlowExpr(action.Duration.Source)
	doSteps, onTimeout := step.Children["_do"], step.Children["_onTimeout"]
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
	doBodyStmts, parseErr := parseFlowStmtList(renderTypedFlowSteps(toState, doSteps, 1) + "return nil\n")
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
		timeoutBranch, parseErr = parseFlowStmtList(renderTypedFlowSteps(cloneFlowState(st), onTimeout, 1))
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

func renderFlowFallbackAST(st *flowRenderState, step flowir.TypedStep, action flowir.FlowFallback, indent int, sfx string) (string, bool) {
	mainSteps, fallbackSteps := step.Children["_do"], step.Children["_fallback"]
	if len(mainSteps) == 0 || len(fallbackSteps) == 0 {
		return "", true
	}

	runV, errV := "_fbRun"+sfx, "_fbErr"+sfx
	stmts := make([]ast.Stmt, 0, 3)

	runState := cloneFlowState(st)
	runState.returnErrOnly = true
	runBody, err := parseFlowStmtList(renderTypedFlowSteps(runState, mainSteps, 1) + "return nil\n")
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

	fallbackBody, parseErr := parseFlowStmtList(renderTypedFlowSteps(cloneFlowState(st), fallbackSteps, 1))
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
