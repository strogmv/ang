package emitter

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowRecordEvent(st *flowRenderState, action flowir.FlowRecordEvent, indent int, sfx string) string {
	pad := strings.Repeat("\t", indent)
	name := normalizeFlowExpr(action.Name.Source)
	if name == "" {
		return ""
	}
	if _, err := parseFlowExprSafe(name); err != nil {
		return ""
	}
	payload := normalizeFlowExpr(action.Payload.Source)
	if payload == "" {
		payload = "nil"
	} else if _, err := parseFlowExprSafe(payload); err != nil {
		return ""
	}

	evVar := "_flowEvt" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := map[string]any{\"seq\": len(_flowHistory) + 1, \"name\": %s, \"payload\": %s}\n", pad, evVar, name, payload))
	b.WriteString(fmt.Sprintf("%sif !_flowReplayMode {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_flowHistory = append(_flowHistory, %s)\n", pad, evVar))
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	if output := action.Output; output != "" {
		assignLine, ok := renderFlowOutputAssign(st, indent, output, evVar, "map[string]any")
		if !ok {
			return ""
		}
		b.WriteString(assignLine)
	}
	return b.String()
}

func renderFlowHistoryGet(st *flowRenderState, action flowir.FlowHistoryGet, indent int, sfx string) string {
	pad := strings.Repeat("\t", indent)
	output := action.Output
	if output == "" {
		return ""
	}
	filterName := normalizeFlowExpr(action.Name.Source)
	if filterName != "" {
		if _, err := parseFlowExprSafe(filterName); err != nil {
			return ""
		}
	}
	limit := normalizeFlowExpr(action.Limit.Source)
	if limit != "" {
		if _, err := parseFlowExprSafe(limit); err != nil {
			return ""
		}
	}

	outVar := "_flowHistOut" + sfx
	itemVar := "_flowHistItem" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := make([]map[string]any, 0, len(_flowHistory))\n", pad, outVar))
	if filterName == "" {
		b.WriteString(fmt.Sprintf("%s%s = append(%s, _flowHistory...)\n", pad, outVar, outVar))
	} else {
		b.WriteString(fmt.Sprintf("%sfor _, %s := range _flowHistory {\n", pad, itemVar))
		b.WriteString(fmt.Sprintf("%s\tif fmt.Sprint(%s[\"name\"]) == fmt.Sprint(%s) {\n", pad, itemVar, filterName))
		b.WriteString(fmt.Sprintf("%s\t\t%s = append(%s, %s)\n", pad, outVar, outVar, itemVar))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	if limit != "" {
		limVar := "_flowHistLimit" + sfx
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, limVar, limit))
		b.WriteString(fmt.Sprintf("%sif %s > 0 && len(%s) > %s {\n", pad, limVar, outVar, limVar))
		b.WriteString(fmt.Sprintf("%s\t%s = %s[len(%s)-%s:]\n", pad, outVar, outVar, outVar, limVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}

	assignLine, ok := renderFlowOutputAssign(st, indent, output, outVar, "[]map[string]any")
	if !ok {
		return ""
	}
	b.WriteString(assignLine)
	return b.String()
}

func renderFlowReplay(st *flowRenderState, action flowir.FlowReplay, indent int, sfx string) string {
	pad := strings.Repeat("\t", indent)
	historyExpr := normalizeFlowExpr(action.History.Source)
	if historyExpr == "" {
		return ""
	}
	if _, err := parseFlowExprSafe(historyExpr); err != nil {
		return ""
	}

	srcVar := "_flowReplaySrc" + sfx
	itemsVar := "_flowReplayItems" + sfx
	okVar := "_flowReplayOK" + sfx
	anyVar := "_flowReplayAny" + sfx
	convVar := "_flowReplayConv" + sfx

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, srcVar, historyExpr))
	b.WriteString(fmt.Sprintf("%s%s, %s := %s.([]map[string]any)\n", pad, itemsVar, okVar, srcVar))
	b.WriteString(fmt.Sprintf("%sif !%s {\n", pad, okVar))
	b.WriteString(fmt.Sprintf("%s\tif %s, _ok := %s.([]any); _ok {\n", pad, anyVar, srcVar))
	b.WriteString(fmt.Sprintf("%s\t\t%s := make([]map[string]any, 0, len(%s))\n", pad, convVar, anyVar))
	b.WriteString(fmt.Sprintf("%s\t\tfor _, _raw := range %s {\n", pad, anyVar))
	b.WriteString(fmt.Sprintf("%s\t\t\tif _m, _ok := _raw.(map[string]any); _ok {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = append(%s, _m)\n", pad, convVar, convVar))
	b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\t%s = %s\n", pad, itemsVar, convVar))
	b.WriteString(fmt.Sprintf("%s\t\t%s = true\n", pad, okVar))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	var onMismatch, doSteps []normalizer.FlowStep

	b.WriteString(fmt.Sprintf("%sif !%s {\n", pad, okVar))
	if flowNestedStepCount(st, "_onMismatch", onMismatch) > 0 {
		b.WriteString(renderFlowNestedSteps(cloneFlowState(st), "_onMismatch", onMismatch, indent+1))
	} else {
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusBadRequest, \"INVALID_REPLAY_HISTORY\", \"flow.Replay history must be []map[string]any\")"))
	}
	b.WriteString(fmt.Sprintf("%s} else {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_flowHistory = append(make([]map[string]any, 0, len(%s)), %s...)\n", pad, itemsVar, itemsVar))

	if output := action.Output; output != "" {
		assignLine, ok := renderFlowOutputAssign(st, indent+1, output, "_flowHistory", "[]map[string]any")
		if !ok {
			return ""
		}
		b.WriteString(assignLine)
	}
	if flowNestedStepCount(st, "_do", doSteps) > 0 {
		prevVar := "_flowReplayPrev" + sfx
		b.WriteString(fmt.Sprintf("%s\t%s := _flowReplayMode\n", pad, prevVar))
		b.WriteString(fmt.Sprintf("%s\t_flowReplayMode = true\n", pad))
		b.WriteString(renderFlowNestedSteps(cloneFlowState(st), "_do", doSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s\t_flowReplayMode = %s\n", pad, prevVar))
	}
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	return b.String()
}

func renderFlowOutputAssign(st *flowRenderState, indent int, output, rhs, typ string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	outExpr, err := parseFlowExprSafe(output)
	if err != nil {
		return "", false
	}
	assign := "="
	if !st.declared[output] {
		if _, ok := outExpr.(*ast.Ident); !ok {
			return "", false
		}
		assign = ":="
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = typ
	}
	return fmt.Sprintf("%s%s %s %s\n", pad, output, assign, rhs), true
}
