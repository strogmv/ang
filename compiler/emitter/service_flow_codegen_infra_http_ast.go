package emitter

import (
	"go/ast"
	"go/token"
	"sort"
	"strconv"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowHTTPCallAST(st *flowRenderState, action flowir.HTTPCall, indent int, sfx string) (string, bool) {
	method, url, body := action.Method, normalizeFlowExpr(action.URL.Source), normalizeFlowExpr(action.Body.Source)
	output, statusVar := action.Output, action.StatusVar
	if method == "" || url == "" {
		return "", true
	}
	failOnError, attempts, backoffMs := action.FailOnError, action.Attempts, action.BackoffMS
	if attempts <= 0 {
		attempts = 2
	}

	urlExpr, err := parseFlowExprSafe(url)
	if err != nil {
		return "", false
	}
	timeoutExprRaw := normalizeFlowExpr(action.Timeout.Source)
	if timeoutExprRaw == "" {
		timeoutExprRaw = "5 * time.Second"
	}
	timeoutExpr, err := parseFlowExprSafe(timeoutExprRaw)
	if err != nil {
		return "", false
	}
	bodyArg := ast.Expr(ast.NewIdent("nil"))
	if body != "" {
		bodyExpr, parseErr := parseFlowExprSafe(body)
		if parseErr != nil {
			return "", false
		}
		bodyArg = &ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: ast.NewIdent("strings"), Sel: ast.NewIdent("NewReader")},
			Args: []ast.Expr{bodyExpr},
		}
	}

	httpReqV, httpReqErrV := "_httpReq"+sfx, "_httpReqErr"+sfx
	httpResV, httpErrV := "_httpRes"+sfx, "_hErr"+sfx
	httpBodyV := "_httpBody" + sfx
	httpTryV := "_httpTry" + sfx
	httpCallCtxV := "_httpCallCtx" + sfx
	httpCancelV := "_httpCancel" + sfx

	stmts := make([]ast.Stmt, 0, 20)
	stmts = append(stmts,
		&ast.DeclStmt{
			Decl: &ast.GenDecl{
				Tok: token.VAR,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names: []*ast.Ident{ast.NewIdent(httpResV)},
						Type:  mustParseExpr("*http.Response"),
					},
				},
			},
		},
		&ast.DeclStmt{
			Decl: &ast.GenDecl{
				Tok: token.VAR,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names: []*ast.Ident{ast.NewIdent(httpErrV)},
						Type:  ast.NewIdent("error"),
					},
				},
			},
		},
	)

	loopBody := make([]ast.Stmt, 0, 8)
	loopBody = append(loopBody, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(httpCallCtxV), ast.NewIdent(httpCancelV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{X: ast.NewIdent("context"), Sel: ast.NewIdent("WithTimeout")},
				Args: []ast.Expr{
					ast.NewIdent("ctx"),
					timeoutExpr,
				},
			},
		},
	})
	loopBody = append(loopBody, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(httpReqV), ast.NewIdent(httpReqErrV)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{X: ast.NewIdent("http"), Sel: ast.NewIdent("NewRequestWithContext")},
				Args: []ast.Expr{
					ast.NewIdent(httpCallCtxV),
					&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(method)},
					urlExpr,
					bodyArg,
				},
			},
		},
	})
	reqErrBody := []ast.Stmt{
		&ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent(httpCancelV)}},
	}
	reqReturnStmts, err := parseFlowStmtList(errReturn(st, "", `fmt.Errorf("http: request: %w", `+httpReqErrV+`)`))
	if err != nil {
		return "", false
	}
	reqErrBody = append(reqErrBody, reqReturnStmts...)
	loopBody = append(loopBody, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(httpReqErrV),
			Op: token.NEQ,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{List: reqErrBody},
	})
	if hdrs := action.Headers; len(hdrs) > 0 {
		keys := make([]string, 0, len(hdrs))
		for hk := range hdrs {
			keys = append(keys, hk)
		}
		sort.Strings(keys)
		for _, hk := range keys {
			hvExpr, parseErr := parseFlowExprSafe(hdrs[hk].Source)
			if parseErr != nil {
				return "", false
			}
			loopBody = append(loopBody, &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   &ast.SelectorExpr{X: ast.NewIdent(httpReqV), Sel: ast.NewIdent("Header")},
						Sel: ast.NewIdent("Set"),
					},
					Args: []ast.Expr{
						&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(hk)},
						hvExpr,
					},
				},
			})
		}
	}
	loopBody = append(loopBody, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(httpResV), ast.NewIdent(httpErrV)},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   &ast.SelectorExpr{X: ast.NewIdent("http"), Sel: ast.NewIdent("DefaultClient")},
					Sel: ast.NewIdent("Do"),
				},
				Args: []ast.Expr{
					ast.NewIdent(httpReqV),
				},
			},
		},
	})
	loopBody = append(loopBody, &ast.ExprStmt{
		X: &ast.CallExpr{Fun: ast.NewIdent(httpCancelV)},
	})
	loopBody = append(loopBody, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(httpErrV),
			Op: token.EQL,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}},
	})
	loopBody = append(loopBody, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X: &ast.BinaryExpr{
				X: &ast.BinaryExpr{
					X:  ast.NewIdent(httpTryV),
					Op: token.ADD,
					Y:  flowIntLit(1),
				},
				Op: token.LSS,
				Y:  flowIntLit(attempts),
			},
			Op: token.LAND,
			Y: &ast.BinaryExpr{
				X:  flowIntLit(backoffMs),
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
									Args: []ast.Expr{flowIntLit(backoffMs)},
								},
								Op: token.MUL,
								Y:  &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Millisecond")},
							},
						},
					},
				},
			},
		},
	})

	stmts = append(stmts, &ast.ForStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(httpTryV)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{flowIntLit(0)},
		},
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(httpTryV),
			Op: token.LSS,
			Y:  flowIntLit(attempts),
		},
		Post: &ast.IncDecStmt{
			X:   ast.NewIdent(httpTryV),
			Tok: token.INC,
		},
		Body: &ast.BlockStmt{List: loopBody},
	})

	httpErrBody, err := parseFlowStmtList(errReturn(st, "", `fmt.Errorf("http: %w", `+httpErrV+`)`))
	if err != nil {
		return "", false
	}
	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(httpErrV),
			Op: token.NEQ,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{List: httpErrBody},
	})
	nilResponseBody, parseErr := parseFlowStmtList(errReturn(st, "", `errors.New(http.StatusBadGateway, "HTTP_ERROR", "empty response")`))
	if parseErr != nil {
		return "", false
	}
	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent(httpResV),
			Op: token.EQL,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{
			List: nilResponseBody,
		},
	})

	stmts = append(stmts, &ast.DeferStmt{
		Call: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.SelectorExpr{X: ast.NewIdent(httpResV), Sel: ast.NewIdent("Body")},
				Sel: ast.NewIdent("Close"),
			},
		},
	})
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(httpBodyV), ast.NewIdent("_")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{X: ast.NewIdent("io"), Sel: ast.NewIdent("ReadAll")},
				Args: []ast.Expr{
					&ast.SelectorExpr{X: ast.NewIdent(httpResV), Sel: ast.NewIdent("Body")},
				},
			},
		},
	})

	if statusVar != "" {
		assignTok := token.DEFINE
		if st.declared[statusVar] {
			assignTok = token.ASSIGN
		}
		st.declared[statusVar] = true
		st.pointers[statusVar] = false
		stmts = append(stmts, &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(statusVar)},
			Tok: assignTok,
			Rhs: []ast.Expr{
				&ast.SelectorExpr{X: ast.NewIdent(httpResV), Sel: ast.NewIdent("StatusCode")},
			},
		})
	}

	if failOnError {
		statusErrBody, parseErr := parseFlowStmtList(errReturn(st, "", `errors.New(`+httpResV+`.StatusCode, "HTTP_ERROR", string(`+httpBodyV+`))`))
		if parseErr != nil {
			return "", false
		}
		stmts = append(stmts, &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X:  &ast.SelectorExpr{X: ast.NewIdent(httpResV), Sel: ast.NewIdent("StatusCode")},
				Op: token.GEQ,
				Y:  flowIntLit(400),
			},
			Body: &ast.BlockStmt{List: statusErrBody},
		})
	}

	if output != "" {
		assignTok := token.DEFINE
		if st.declared[output] {
			assignTok = token.ASSIGN
		}
		st.declared[output] = true
		st.pointers[output] = false
		stmts = append(stmts, &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(output)},
			Tok: assignTok,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun:  ast.NewIdent("string"),
					Args: []ast.Expr{ast.NewIdent(httpBodyV)},
				},
			},
		})
	}

	return renderFlowASTStmts(stmts, indent), true
}
