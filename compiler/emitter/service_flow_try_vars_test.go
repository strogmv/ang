package emitter

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// tryDeclaringTotal is a flow.Try whose body declares `total`, and a step
// after the try that reads it (backlog 41).
func tryDeclaringTotal(extraBodySteps ...normalizer.FlowStep) []normalizer.FlowStep {
	body := append([]normalizer.FlowStep{
		{Action: "mapping.Assign", Args: map[string]any{"to": "total", "declare": "true", "value": "len(req.Items)"}},
	}, extraBodySteps...)
	return []normalizer.FlowStep{
		{Action: "flow.Try", Args: map[string]any{"retries": 1, "_do": body}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Total", "value": "total"}},
	}
}

func TestFlowTryDeclaresBodyVariablesWhereTheNextStepsSeeThem(t *testing.T) {
	code := renderFlow(tryDeclaringTotal())
	stmts, err := parseFlowStmtList(code)
	if err != nil {
		t.Fatalf("generated flow does not parse: %v\n%s", err, code)
	}
	declaredAtTop := false
	for _, stmt := range stmts {
		decl, ok := stmt.(*ast.DeclStmt)
		if !ok {
			continue
		}
		for _, spec := range decl.Decl.(*ast.GenDecl).Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				if name.Name == "total" {
					declaredAtTop = true
				}
			}
		}
	}
	if !declaredAtTop {
		t.Fatalf("`var total` must be declared in the flow's scope, not inside the try block:\n%s", code)
	}
	if strings.Count(code, "var total ") != 1 || strings.Contains(code, "total :=") {
		t.Fatalf("total must be declared exactly once and assigned in the body:\n%s", code)
	}
	if !strings.Contains(code, "helpers.Assign(&total, len(req.Items))") || !strings.Contains(code, "helpers.Assign(&resp.Total, total)") {
		t.Fatalf("body must assign the declared total and the next step read it:\n%s", code)
	}
}

func TestFlowTryTextFallbackDeclaresBodyVariables(t *testing.T) {
	// A body that does not parse makes renderFlowTryAST give up; the text
	// renderer must still declare total (it used to get nil step lists and
	// the AST renderer's half-applied state, and declared nothing).
	broken := normalizer.FlowStep{Action: "mapping.Assign", Args: map[string]any{"to": "broken", "declare": "true", "value": "count(("}}
	code := renderFlow(tryDeclaringTotal(broken))
	if !strings.Contains(code, "_tryRun_") || !strings.Contains(code, "count((") {
		t.Fatalf("expected the text renderer's output:\n%s", code)
	}
	lines := strings.Split(code, "\n")
	open := -1
	for i, line := range lines {
		if line == "{" {
			open = i
			break
		}
	}
	if open < 0 {
		t.Fatalf("no try block in:\n%s", code)
	}
	head := strings.Join(lines[:open], "\n")
	for _, want := range []string{"var total any", "var broken any"} {
		if !strings.Contains(head, want) {
			t.Fatalf("%q must come before the try block:\n%s", want, code)
		}
	}
	if strings.Count(code, "var total ") != 1 {
		t.Fatalf("total must be declared once:\n%s", code)
	}
	if !strings.Contains(code, "helpers.Assign(&resp.Total, total)") {
		t.Fatalf("the step after the try must read total:\n%s", code)
	}
}
