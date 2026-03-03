package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepSaga(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "flow.Saga":
		doSteps := child("_do")
		if len(doSteps) == 0 {
			return "", true
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
		b.WriteString(renderFlowSteps(sagaState, doSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "flow.Compensate":
		doSteps := child("_do")
		if len(doSteps) == 0 || st.sagaCompVar == "" {
			return "", true
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s = append(%s, func(ctx context.Context) error {\n", pad, st.sagaCompVar, st.sagaCompVar))
		
		compState := cloneFlowState(st)
		compState.returnErrOnly = true
		b.WriteString(renderFlowSteps(compState, doSteps, indent+1))
		
		b.WriteString(fmt.Sprintf("%s\treturn nil\n", pad))
		b.WriteString(fmt.Sprintf("%s})\n", pad))
		return b.String(), true

	case "flow.Rollback":
		errExpr := arg("error")
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
