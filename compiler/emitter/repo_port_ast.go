package emitter

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

type methodParam struct {
	name string
	typ  string
}

func (e *Emitter) renderRepositoryPortAST(repo normalizer.Repository) ([]byte, error) {
	entityName := ExportName(repo.Entity)
	if entityName == "" {
		return nil, fmt.Errorf("repository entity is empty")
	}

	hasListAll := false
	hasTime := false
	methods := make([]*ast.Field, 0, 4+len(repo.Finders))

	methods = append(methods, buildMethodField("Save",
		[]methodParam{{name: "ctx", typ: "context.Context"}, {name: "entity", typ: "*domain." + entityName}},
		[]string{"error"},
	))
	methods = append(methods, buildMethodField("FindByID",
		[]methodParam{{name: "ctx", typ: "context.Context"}, {name: "id", typ: "string"}},
		[]string{"*domain." + entityName, "error"},
	))
	methods = append(methods, buildMethodField("Delete",
		[]methodParam{{name: "ctx", typ: "context.Context"}, {name: "id", typ: "string"}},
		[]string{"error"},
	))

	for _, f := range repo.Finders {
		sig := ComputeFinderSignature(repo.Entity, f, "")
		hasTime = hasTime || sig.HasTime
		if ExportName(f.Name) == "ListAll" {
			hasListAll = true
		}
		params := []methodParam{{name: "ctx", typ: "context.Context"}}
		for _, w := range f.Where {
			goType, isTime := finderParamType(w.ParamType)
			hasTime = hasTime || isTime
			paramName := strings.TrimSpace(w.Param)
			if paramName == "" {
				paramName = strings.ToLower(ExportName(w.Field))
				if paramName == "" {
					paramName = "arg"
				}
			}
			params = append(params, methodParam{name: paramName, typ: goType})
		}
		// Inject pagination params when finder defines Limit (for List-style finders)
		if f.Limit > 0 || strings.Contains(strings.ToLower(f.Name), "paginate") {
			params = append(params, methodParam{name: "limit", typ: "int"})
			params = append(params, methodParam{name: "offset", typ: "int"})
		}
		methods = append(methods, buildMethodField(ExportName(f.Name), params, []string{sig.ReturnType, "error"}))
	}

	if !hasListAll {
		methods = append(methods, buildMethodField("ListAll",
			[]methodParam{{name: "ctx", typ: "context.Context"}, {name: "offset", typ: "int"}, {name: "limit", typ: "int"}},
			[]string{"[]domain." + entityName, "error"},
		))
	}

	importSpecs := []ast.Spec{
		&ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: `"context"`}},
		&ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", e.GoModule+"/internal/domain")}},
	}
	if hasTime {
		importSpecs = append(importSpecs, &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: `"time"`}})
	}

	file := &ast.File{
		Name: ast.NewIdent("port"),
		Decls: []ast.Decl{
			&ast.GenDecl{
				Tok: token.IMPORT,
				Doc: &ast.CommentGroup{List: []*ast.Comment{
					{Text: "//go:generate go run go.uber.org/mock/mockgen@latest -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks"},
				}},
				Specs: importSpecs,
			},
			&ast.GenDecl{
				Tok: token.TYPE,
				Specs: []ast.Spec{
					&ast.TypeSpec{
						Name: ast.NewIdent(entityName + "Repository"),
						Type: &ast.InterfaceType{
							Methods: &ast.FieldList{List: methods},
						},
					},
				},
				Doc: &ast.CommentGroup{List: []*ast.Comment{
					{Text: fmt.Sprintf("// %sRepository defines storage operations for %s", entityName, entityName)},
				}},
			},
		},
	}

	var out bytes.Buffer
	fset := token.NewFileSet()
	if err := format.Node(&out, fset, file); err != nil {
		return nil, fmt.Errorf("format ast: %w", err)
	}
	return out.Bytes(), nil
}

func buildMethodField(name string, params []methodParam, returns []string) *ast.Field {
	paramFields := make([]*ast.Field, 0, len(params))
	for _, p := range params {
		paramFields = append(paramFields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(p.name)},
			Type:  mustParseExpr(p.typ),
		})
	}

	resultFields := make([]*ast.Field, 0, len(returns))
	for _, r := range returns {
		resultFields = append(resultFields, &ast.Field{Type: mustParseExpr(r)})
	}

	return &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(name)},
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: paramFields},
			Results: &ast.FieldList{List: resultFields},
		},
	}
}

func mustParseExpr(src string) ast.Expr {
	expr, err := parser.ParseExpr(src)
	if err != nil {
		panic(fmt.Sprintf("invalid generated type expression %q: %v", src, err))
	}
	return expr
}
