package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepSaga renders saga nodes straight from TypedStep.Children.
// The legacy renderer below remains for compatibility-only paths that start
// with normalizer.FlowStep; typed production dispatch never reconstructs that
// representation for a saga tree.
func renderTypedStepSaga(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "flow.Saga":
		if _, err := typedActionAs[flowir.FlowSaga](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		compVar := "_sagaCompensations" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\tvar %s []func(context.Context) error\n", pad, compVar))
		b.WriteString(fmt.Sprintf("%s\tdefer func() {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif err != nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tfor i := len(%s) - 1; i >= 0; i-- {\n", pad, compVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t_ = %s[i](ctx)\n", pad, compVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}()\n", pad))

		sagaState := cloneFlowState(st)
		sagaState.sagaCompVar = compVar
		b.WriteString(renderTypedFlowSteps(sagaState, step.Children["_do"], indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "flow.Compensate":
		if _, err := typedActionAs[flowir.FlowCompensate](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		if st.sagaCompVar == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s = append(%s, func(ctx context.Context) error {\n", pad, st.sagaCompVar, st.sagaCompVar))
		compState := cloneFlowState(st)
		compState.returnErrOnly = true
		b.WriteString(renderTypedFlowSteps(compState, step.Children["_do"], indent+1))
		b.WriteString(fmt.Sprintf("%s\treturn nil\n", pad))
		b.WriteString(fmt.Sprintf("%s})\n", pad))
		return b.String(), true

	case "flow.Rollback":
		action, err := typedActionAs[flowir.FlowRollback](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		errExpr := normalizeFlowExpr(action.Error.Source)
		if errExpr == "" {
			errExpr = "fmt.Errorf(\"saga rollback triggered\")"
		}
		return fmt.Sprintf("%serr = %s\n%s", pad, errExpr, errReturn(st, pad, "err")), true
	}
	return "", false
}

func renderFlowStepSaga(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "flow.Saga":
		_, decodeErr := decodeCurrentActionAs[flowir.FlowSaga](st, step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, decodeErr.Error()), true
		}
		var doSteps []normalizer.FlowStep

		compVar := "_sagaCompensations" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\tvar %s []func(context.Context) error\n", pad, compVar))
		b.WriteString(fmt.Sprintf("%s\tdefer func() {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif err != nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tfor i := len(%s) - 1; i >= 0; i-- {\n", pad, compVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t_ = %s[i](ctx)\n", pad, compVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}()\n", pad))

		sagaState := cloneFlowState(st)
		sagaState.sagaCompVar = compVar
		b.WriteString(renderFlowNestedSteps(sagaState, "_do", doSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "flow.Compensate":
		_, decodeErr := decodeCurrentActionAs[flowir.FlowCompensate](st, step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, decodeErr.Error()), true
		}
		var doSteps []normalizer.FlowStep
		if st.sagaCompVar == "" {
			return "", true
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s = append(%s, func(ctx context.Context) error {\n", pad, st.sagaCompVar, st.sagaCompVar))

		compState := cloneFlowState(st)
		compState.returnErrOnly = true
		b.WriteString(renderFlowNestedSteps(compState, "_do", doSteps, indent+1))

		b.WriteString(fmt.Sprintf("%s\treturn nil\n", pad))
		b.WriteString(fmt.Sprintf("%s})\n", pad))
		return b.String(), true

	case "flow.Rollback":
		typed, decodeErr := decodeCurrentActionAs[flowir.FlowRollback](st, step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, decodeErr.Error()), true
		}
		errExpr := normalizeFlowExpr(typed.Error.Source)
		if errExpr == "" {
			errExpr = "fmt.Errorf(\"saga rollback triggered\")"
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%serr = %s\n", pad, errExpr))
		b.WriteString(errReturn(st, pad, "err"))
		return b.String(), true
	}

	return "", false
}
