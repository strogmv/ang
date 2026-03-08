package emitter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
)

// TestFlowStepCoverageComplete verifies a stricter contract than schema parity:
// every emitter-supported action must be reachable from #FlowStep union.
func TestFlowStepCoverageComplete(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	supported := parseFlowActionSupported(t, filepath.Join(root, "compiler", "emitter", "service_flow_codegen.go"))
	unionActions := parseFlowStepUnionActions(t, filepath.Join(root, "cue", "schema", "types.cue"))

	var missing []string
	for action := range supported {
		if _, ok := unionActions[action]; ok {
			continue
		}
		missing = append(missing, action)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("flowActionSupported/#FlowStep mismatch; actions not reachable via #FlowStep: %s", strings.Join(missing, ", "))
	}
}

func parseFlowStepUnionActions(t *testing.T, path string) map[string]struct{} {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	f, err := parser.ParseFile(path, src)
	if err != nil {
		t.Fatalf("parse cue file %s: %v", path, err)
	}

	var flowStepExpr ast.Expr
	stepDefs := make(map[string]ast.Expr, 256)
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		label := cueLabel(fd.Label)
		if label == "" {
			continue
		}
		if label == "#FlowStep" {
			flowStepExpr = fd.Value
		}
		stepDefs[label] = fd.Value
	}
	if flowStepExpr == nil {
		t.Fatal("could not find #FlowStep in cue/schema/types.cue")
	}

	stepNames := make(map[string]struct{}, 256)
	collectUnionStepNames(flowStepExpr, stepNames)
	if len(stepNames) == 0 {
		t.Fatal("could not extract any step names from #FlowStep union")
	}

	actions := make(map[string]struct{}, 256)
	for step := range stepNames {
		expr, ok := stepDefs[step]
		if !ok {
			continue
		}
		for action := range collectActionLiteralsFromExpr(expr) {
			actions[action] = struct{}{}
		}
	}
	return actions
}

func collectUnionStepNames(expr ast.Expr, out map[string]struct{}) {
	switch n := expr.(type) {
	case *ast.BinaryExpr:
		if n.Op == token.OR {
			collectUnionStepNames(n.X, out)
			collectUnionStepNames(n.Y, out)
		}
	case *ast.ParenExpr:
		collectUnionStepNames(n.X, out)
	case *ast.Ident:
		out[n.Name] = struct{}{}
	}
}

func collectActionLiteralsFromExpr(expr ast.Expr) map[string]struct{} {
	out := make(map[string]struct{}, 4)
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.StructLit:
			for _, elt := range n.Elts {
				fd, ok := elt.(*ast.Field)
				if !ok {
					continue
				}
				if cueLabel(fd.Label) != "action" {
					continue
				}
				collectStringLits(fd.Value, out)
			}
		case *ast.BinaryExpr:
			if n.Op == token.OR {
				walk(n.X)
				walk(n.Y)
			}
		case *ast.ParenExpr:
			walk(n.X)
		}
	}
	walk(expr)
	return out
}

func collectStringLits(expr ast.Expr, out map[string]struct{}) {
	switch n := expr.(type) {
	case *ast.BinaryExpr:
		if n.Op == token.OR {
			collectStringLits(n.X, out)
			collectStringLits(n.Y, out)
		}
	case *ast.ParenExpr:
		collectStringLits(n.X, out)
	case *ast.BasicLit:
		if n.Kind != token.STRING {
			return
		}
		s, err := strconv.Unquote(n.Value)
		if err != nil {
			panic(fmt.Sprintf("invalid string literal %q: %v", n.Value, err))
		}
		out[s] = struct{}{}
	}
}

func cueLabel(l ast.Label) string {
	switch v := l.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s, err := strconv.Unquote(v.Value)
			if err == nil {
				return s
			}
			return strings.Trim(v.Value, `"`)
		}
		return v.Value
	default:
		return ""
	}
}
