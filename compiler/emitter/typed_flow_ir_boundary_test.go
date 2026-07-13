package emitter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTypedEmitterDoesNotReadLegacyScalarArgs protects the Typed Flow IR
// boundary. ScalarArgs may remain on TypedStep temporarily for compatibility
// diagnostics, but production emission must use Action, Children and Branches.
func TestTypedEmitterDoesNotReadLegacyScalarArgs(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(repoRoot(t), "compiler", "emitter")
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read emitter directory: %v", err)
	}
	var offenders []string
	for _, entry := range files {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "ScalarArgs" {
				offenders = append(offenders, name)
			}
			return true
		})
	}
	if len(offenders) > 0 {
		t.Fatalf("typed emitter must not read legacy ScalarArgs: %s", strings.Join(offenders, ", "))
	}
}

// TestTypedDispatchDoesNotCallLegacyRenderers makes the production boundary
// explicit: compatibility renderers may remain while tests are migrated, but
// renderTypedStepDispatch must never reach them.
func TestTypedDispatchDoesNotCallLegacyRenderers(t *testing.T) {
	t.Parallel()

	path := filepath.Join(repoRoot(t), "compiler", "emitter", "service_flow_codegen.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse typed dispatcher: %v", err)
	}
	var dispatch *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "renderTypedStepDispatch" {
			dispatch = fn
			break
		}
	}
	if dispatch == nil {
		t.Fatal("renderTypedStepDispatch not found")
	}
	legacyNames := map[string]bool{
		"decodeCurrentActionAs":    true,
		"flowStepMetadata":         true,
		"renderLegacyStepDispatch": true,
	}
	var offenders []string
	ast.Inspect(dispatch.Body, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && (legacyNames[ident.Name] || strings.HasPrefix(ident.Name, "renderFlowStep")) {
			offenders = append(offenders, ident.Name)
		}
		return true
	})
	if len(offenders) > 0 {
		t.Fatalf("typed dispatcher calls legacy rendering: %s", strings.Join(offenders, ", "))
	}
}

func TestTypedControlFlowBasicDoesNotCallRawHelpers(t *testing.T) {
	t.Parallel()

	path := filepath.Join(repoRoot(t), "compiler", "emitter", "service_flow_codegen_control_flow_typed.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse typed control flow: %v", err)
	}
	var basic *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "renderTypedStepControlFlowBasic" {
			basic = fn
			break
		}
	}
	if basic == nil {
		t.Fatal("renderTypedStepControlFlowBasic not found")
	}
	var offenders []string
	ast.Inspect(basic.Body, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && strings.HasPrefix(ident.Name, "renderFlow") {
			offenders = append(offenders, ident.Name)
		}
		return true
	})
	if len(offenders) > 0 {
		t.Fatalf("typed control flow calls raw helper(s): %s", strings.Join(offenders, ", "))
	}
}

func TestTypedControlFlowStatefulDoesNotCallLegacyFallbacks(t *testing.T) {
	t.Parallel()

	path := filepath.Join(repoRoot(t), "compiler", "emitter", "service_flow_codegen_control_flow_typed.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse typed control flow: %v", err)
	}
	var stateful *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "renderTypedStepControlFlowStateful" {
			stateful = fn
			break
		}
	}
	if stateful == nil {
		t.Fatal("renderTypedStepControlFlowStateful not found")
	}
	legacyNames := map[string]bool{
		"renderFlowCheckpointAST":      true,
		"renderFlowCheckpointLegacy":   true,
		"renderFlowResumeLegacy":       true,
		"renderFlowValidateAST":        true,
		"renderFlowValidateLegacy":     true,
		"renderFlowCatchAST":           true,
		"renderFlowCatchLegacy":        true,
		"renderFlowDeferAST":           true,
		"renderFlowDeferLegacy":        true,
		"renderFlowSuggestNextAST":     true,
		"renderFlowSuggestNextLegacy":  true,
		"renderFlowExplainErrorAST":    true,
		"renderFlowExplainErrorLegacy": true,
	}
	var offenders []string
	ast.Inspect(stateful.Body, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && legacyNames[ident.Name] {
			offenders = append(offenders, ident.Name)
		}
		return true
	})
	if len(offenders) > 0 {
		t.Fatalf("typed stateful control flow calls legacy fallback(s): %s", strings.Join(offenders, ", "))
	}
}
