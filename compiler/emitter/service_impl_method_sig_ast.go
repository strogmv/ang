package emitter

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderServiceImplMethodSignature(serviceName string, m normalizer.Method) (string, error) {
	params := []*ast.Field{
		{
			Names: []*ast.Ident{ast.NewIdent("ctx")},
			Type:  mustParseExpr("context.Context"),
		},
	}

	eventName := ""
	if len(m.Publishes) > 0 && m.Input.Name == "" && m.Output.Name == "" {
		eventName = m.Publishes[0]
	}

	if eventName != "" {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("req")},
			Type:  mustParseExpr("domain." + ExportName(eventName)),
		})
	} else {
		reqType := strings.TrimSpace(m.Input.Name)
		if reqType == "" {
			reqType = "struct{}"
		}
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("req")},
			Type:  mustParseExpr("port." + reqType),
		})
	}

	results := []*ast.Field{}
	if m.Output.Name != "" && eventName == "" {
		results = append(results, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("resp")},
			Type:  mustParseExpr("port." + m.Output.Name),
		})
		results = append(results, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("err")},
			Type:  ast.NewIdent("error"),
		})
	} else {
		// Preserve existing template behavior for no-output methods:
		// both named returns are error (resp, err).
		results = append(results, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("resp")},
			Type:  ast.NewIdent("error"),
		})
		results = append(results, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("err")},
			Type:  ast.NewIdent("error"),
		})
	}

	fd := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("s")},
					Type:  mustParseExpr("*" + serviceName + "Impl"),
				},
			},
		},
		Name: ast.NewIdent(m.Name),
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: params},
			Results: &ast.FieldList{List: results},
		},
		Body: &ast.BlockStmt{},
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), fd); err != nil {
		return "", fmt.Errorf("format service impl method signature %s.%s: %w", serviceName, m.Name, err)
	}

	src := buf.String()
	idx := strings.Index(src, "{")
	if idx < 0 {
		return "", fmt.Errorf("unexpected formatted function without body for %s.%s", serviceName, m.Name)
	}
	return strings.TrimSpace(src[:idx]), nil
}
