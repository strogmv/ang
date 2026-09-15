package compiler

import (
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"go/types"

	"github.com/strogmv/ang-ir/normalizer"
)

// emitTautologicalConditionDiagnostics reports conditions that cannot change
// their value: "x || !x" is always true, so a logic.Check with it never
// fails; "x && !x" is always false, so a flow.If with it never runs its
// branch. dealingi-back had such a check guarding webhook dispatch.
func emitTautologicalConditionDiagnostics(services []normalizer.Service, opts PipelineOptions) {
	for _, svc := range services {
		for _, method := range svc.Methods {
			walkFlowSteps(method.Flow, func(step normalizer.FlowStep) {
				if step.Action != "logic.Check" && step.Action != "flow.If" {
					return
				}
				condition, _ := step.Args["condition"].(string)
				verdict := tautologyVerdict(condition)
				if verdict == "" {
					return
				}
				recordPipelineDiagnostic(normalizer.Warning{
					Kind:     "flow",
					Code:     "TAUTOLOGICAL_CHECK",
					Severity: "warn",
					Message:  fmt.Sprintf("%s.%s: %s condition %q is %s", svc.Name, method.Name, step.Action, condition, verdict),
					Hint:     "A check that cannot fail (or a branch that cannot run) is dead code. If it only exists to use a call's output, drop output: from the call instead — the call's error is still checked.",
					File:     step.File,
					Line:     step.Line,
					Column:   step.Column,
					CUEPath:  step.CUEPath,
				}, opts)
			})
		}
	}
}

// tautologyVerdict returns "always true" or "always false" when the top-level
// condition is x || !x or x && !x (in either order), and "" otherwise.
func tautologyVerdict(condition string) string {
	expr, err := goparser.ParseExpr(condition)
	if err != nil {
		return ""
	}
	for {
		paren, ok := expr.(*goast.ParenExpr)
		if !ok {
			break
		}
		expr = paren.X
	}
	binary, ok := expr.(*goast.BinaryExpr)
	if !ok || (binary.Op != gotoken.LOR && binary.Op != gotoken.LAND) {
		return ""
	}
	if !negatesEachOther(binary.X, binary.Y) && !negatesEachOther(binary.Y, binary.X) {
		return ""
	}
	if binary.Op == gotoken.LOR {
		return "always true"
	}
	return "always false"
}

func negatesEachOther(value, negated goast.Expr) bool {
	unary, ok := unparen(negated).(*goast.UnaryExpr)
	if !ok || unary.Op != gotoken.NOT {
		return false
	}
	return types.ExprString(unparen(value)) == types.ExprString(unparen(unary.X))
}

func unparen(expr goast.Expr) goast.Expr {
	for {
		paren, ok := expr.(*goast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}
