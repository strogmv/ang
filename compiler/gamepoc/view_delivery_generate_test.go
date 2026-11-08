package gamepoc

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"testing"
	"text/template"
)

// Optional WebSocket tests must not break a subscription-enabled gRPC-only host.
func TestViewDeliveryTemplateOptionalWebSocket(t *testing.T) {
	raw, err := sources.ReadFile("templates/view_delivery_test.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("delivery").Delims("<<ANG", "ANG>>").Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		var out bytes.Buffer
		if err := tmpl.Execute(&out, Compiled{ViewDelivery: true, WebSocket: enabled}); err != nil {
			t.Fatal(err)
		}
		code, err := format.Source(out.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), "delivery.go", code, 0)
		if err != nil {
			t.Fatal(err)
		}
		selectors := map[string]bool{}
		for _, decl := range file.Decls {
			if _, ok := decl.(*ast.GenDecl); ok {
				continue
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				if s, ok := n.(*ast.SelectorExpr); ok {
					if id, ok := s.X.(*ast.Ident); ok {
						selectors[id.Name] = true
					}
				}
				return true
			})
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			name = path.Base(name)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			if !selectors[name] {
				t.Errorf("websocket=%v: unused generated import %s", enabled, name)
			}
		}
	}
}
