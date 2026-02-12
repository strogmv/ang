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

func renderServiceInterfaceDecl(svc normalizer.Service) (string, error) {
	methods := make([]*ast.Field, 0, len(svc.Methods))
	for _, m := range svc.Methods {
		field, err := buildServiceMethodField(svc, m)
		if err != nil {
			return "", err
		}
		methods = append(methods, field)
	}

	iface := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent(svc.Name),
				Type: &ast.InterfaceType{
					Methods: &ast.FieldList{List: methods},
				},
			},
		},
	}

	var buf bytes.Buffer
	fset := token.NewFileSet()
	if err := format.Node(&buf, fset, iface); err != nil {
		return "", fmt.Errorf("format service interface %s: %w", svc.Name, err)
	}
	return buf.String(), nil
}

func buildServiceMethodField(svc normalizer.Service, m normalizer.Method) (*ast.Field, error) {
	params := []*ast.Field{
		{
			Names: []*ast.Ident{ast.NewIdent("ctx")},
			Type:  mustParseExpr("context.Context"),
		},
	}

	results := []*ast.Field{}
	eventName := ""
	if len(m.Publishes) > 0 && m.Input.Name == "" && m.Output.Name == "" {
		eventName = m.Publishes[0]
	}
	if eventName != "" {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("req")},
			Type:  mustParseExpr("domain." + ExportName(eventName)),
		})
		results = append(results, &ast.Field{Type: ast.NewIdent("error")})
	} else {
		inType := strings.TrimSpace(m.Input.Name)
		if inType == "" {
			inType = "struct{}"
		}
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("req")},
			Type:  mustParseExpr(inType),
		})
		if m.Output.Name != "" {
			results = append(results, &ast.Field{Type: mustParseExpr(m.Output.Name)})
		}
		results = append(results, &ast.Field{Type: ast.NewIdent("error")})
	}

	field := &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(m.Name)},
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: params},
			Results: &ast.FieldList{List: results},
		},
	}

	return field, nil
}
