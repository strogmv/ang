package emitter

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

func parseFlowExprSafe(src string) (ast.Expr, error) {
	return parser.ParseExpr(src)
}

func parseFlowStmtList(src string) ([]ast.Stmt, error) {
	wrapped := "package p\nfunc _gen_(){\n" + src + "\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", wrapped, parser.AllErrors)
	if err != nil {
		return nil, err
	}
	if len(file.Decls) != 1 {
		return nil, fmt.Errorf("unexpected decl count: %d", len(file.Decls))
	}
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		return nil, fmt.Errorf("unexpected wrapper body")
	}
	return fn.Body.List, nil
}

func renderFlowASTStmt(stmt ast.Stmt, indent int) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), stmt); err != nil {
		return ""
	}
	code := strings.TrimRight(buf.String(), "\n")
	return indentFlowCode(code, indent) + "\n"
}

func renderFlowASTStmts(stmts []ast.Stmt, indent int) string {
	if len(stmts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, st := range stmts {
		b.WriteString(renderFlowASTStmt(st, indent))
	}
	return b.String()
}

func indentFlowCode(code string, indent int) string {
	if code == "" || indent <= 0 {
		return code
	}
	pad := strings.Repeat("\t", indent)
	lines := strings.Split(code, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}

func flowIntArg(args map[string]any, key string, def int) int {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return def
	}
}

func flowIntLit(n int) ast.Expr {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(n)}
}

func flowIsAssignableTarget(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	return strings.Contains(target, ".") || strings.Contains(target, "[")
}

func renderFlowAssignTarget(st *flowRenderState, pad, target, expr, typ string) string {
	if strings.TrimSpace(target) == "" || strings.TrimSpace(expr) == "" {
		return ""
	}
	if flowIsAssignableTarget(target) {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, target, expr))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()
	}
	assign := ":="
	if st.declared[target] {
		assign = "="
	}
	st.declared[target] = true
	st.pointers[target] = false
	if strings.TrimSpace(typ) != "" {
		st.types[target] = typ
	}
	return fmt.Sprintf("%s%s %s %s\n", pad, target, assign, expr)
}
