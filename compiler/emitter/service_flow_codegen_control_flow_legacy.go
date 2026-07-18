package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowBlockLegacy(st *flowRenderState, indent int, child func(string) []normalizer.FlowStep) string {
	return renderFlowChildSteps(st, child, "_do", indent)
}

func renderFlowIfLegacy(st *flowRenderState, pad string, indent int, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	cond := arg("condition")
	if cond == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif %s {\n", pad, cond))
	b.WriteString(renderFlowChildSteps(cloneFlowState(st), child, "_then", indent+1))
	b.WriteString(fmt.Sprintf("%s}", pad))
	if flowChildStepCount(st, child, "_else") > 0 {
		b.WriteString(" else {\n")
		b.WriteString(renderFlowChildSteps(cloneFlowState(st), child, "_else", indent+1))
		b.WriteString(fmt.Sprintf("%s}", pad))
	}
	b.WriteString("\n")
	return b.String()
}

func renderFlowForLegacy(st *flowRenderState, pad string, indent int, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	each := arg("each")
	as := arg("as")
	if each == "" {
		return ""
	}
	if as == "" {
		as = "item"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, each))
	b.WriteString(renderFlowChildSteps(cloneFlowState(st), child, "_do", indent+1))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowSwitchLegacy(st *flowRenderState, cases map[string][]normalizer.FlowStep, pad string, indent int, arg func(string) string, defaultSteps []normalizer.FlowStep) string {
	value := arg("value")
	if value == "" {
		return ""
	}
	matchMode := strings.ToLower(strings.TrimSpace(arg("match")))
	if matchMode == "" {
		matchMode = "exact"
	}
	keys := flowBranchNames(st, cases)
	var b strings.Builder
	if matchMode == "exact" {
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, value))
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("%scase %q:\n", pad, k))
			b.WriteString(renderFlowBranchSteps(cloneFlowState(st), k, cases[k], indent+1))
		}
		if flowNestedStepCount(st, "_default", defaultSteps) > 0 {
			b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
			b.WriteString(renderFlowNestedSteps(cloneFlowState(st), "_default", defaultSteps, indent+1))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()
	}
	selector := "_switchValue"
	b.WriteString(fmt.Sprintf("%s%s := strings.TrimSpace(fmt.Sprint(%s))\n", pad, selector, value))
	for i, k := range keys {
		cond := fmt.Sprintf("%s == %q", selector, k)
		switch matchMode {
		case "prefix":
			cond = fmt.Sprintf("strings.HasPrefix(%s, %q)", selector, k)
		case "suffix":
			cond = fmt.Sprintf("strings.HasSuffix(%s, %q)", selector, k)
		case "contains":
			cond = fmt.Sprintf("strings.Contains(%s, %q)", selector, k)
		case "glob":
			cond = fmt.Sprintf("_switchMatch, _ := path.Match(%q, %s); _switchMatch", k, selector)
		}
		if i == 0 {
			b.WriteString(fmt.Sprintf("%sif %s {\n", pad, cond))
		} else {
			b.WriteString(fmt.Sprintf("%s} else if %s {\n", pad, cond))
		}
		b.WriteString(renderFlowBranchSteps(cloneFlowState(st), k, cases[k], indent+1))
	}
	if flowNestedStepCount(st, "_default", defaultSteps) > 0 {
		if len(keys) > 0 {
			b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		} else {
			b.WriteString(fmt.Sprintf("%s{\n", pad))
		}
		b.WriteString(renderFlowNestedSteps(cloneFlowState(st), "_default", defaultSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	} else if len(keys) > 0 {
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	return b.String()
}

func renderFlowWhileLegacy(st *flowRenderState, pad string, indent int, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	cond := arg("condition")
	if cond == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sfor %s {\n", pad, cond))
	b.WriteString(renderFlowChildSteps(cloneFlowState(st), child, "_do", indent+1))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderTypedFlowCheckpoint(pad string, action flowir.FlowCheckpoint) string {
	name := action.Name
	if name == "" {
		return ""
	}
	data := normalizeFlowExpr(action.Data.Source)
	keyLit := fmt.Sprintf("%q", name)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif _flowCheckpoints == nil {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_flowCheckpoints = make(map[string]any)\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s_flowCheckpoints[%s] = %s\n", pad, keyLit, data))
	return b.String()
}

func renderTypedFlowResume(st *flowRenderState, step flowir.TypedStep, action flowir.FlowResume, pad string, indent int, sfx string) string {
	name := action.Name
	if name == "" {
		return ""
	}
	output, into := action.Output, action.Into
	keyLit := fmt.Sprintf("%q", name)
	ckptValV, ckptOKV := "_ckptVal"+sfx, "_ckptOK"+sfx
	ckptCastV, ckptCastOKV := "_ckptCast"+sfx, "_ckptCastOK"+sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%svar %s any\n", pad, ckptValV))
	b.WriteString(fmt.Sprintf("%s%s := false\n", pad, ckptOKV))
	b.WriteString(fmt.Sprintf("%sif _flowCheckpoints != nil {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s, %s = _flowCheckpoints[%s]\n", pad, ckptValV, ckptOKV, keyLit))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sif !%s {\n", pad, ckptOKV))
	if onMissing := step.Children["_onMissing"]; len(onMissing) > 0 {
		b.WriteString(renderTypedFlowSteps(cloneFlowState(st), onMissing, indent+1))
	} else {
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"CHECKPOINT_NOT_FOUND\", \"checkpoint %s not found\")", name)))
	}
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	if output != "" {
		outputType := resolveFlowDynamicOutputType(st, output, into)
		declareOutput := !st.declared[output]
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = outputType
		if outputType == "any" {
			assign := ":="
			if !declareOutput {
				assign = "="
			}
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, ckptValV))
			return b.String()
		}
		if declareOutput {
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, outputType))
		}
		b.WriteString(fmt.Sprintf("%s%s, %s := %s.(%s)\n", pad, ckptCastV, ckptCastOKV, ckptValV, outputType))
		b.WriteString(fmt.Sprintf("%sif !%s {\n", pad, ckptCastOKV))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"flow.Resume %s: checkpoint payload is not %s\")", name, outputType)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, output, ckptCastV))
	}
	return b.String()
}

func renderTypedFlowCatch(st *flowRenderState, step flowir.TypedStep, pad string, indent int) string {
	catchSteps := step.Children["_do"]
	if len(catchSteps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif _flowLastError != nil {\n", pad))
	b.WriteString(renderTypedFlowSteps(cloneFlowState(st), catchSteps, indent+1))
	b.WriteString(fmt.Sprintf("%s\t_flowLastError = nil\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderTypedFlowSuggestNext(st *flowRenderState, action flowir.FlowSuggestNext, pad string) string {
	output, options := action.Output, action.Options
	if len(options) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(options))
	for _, opt := range options {
		quoted = append(quoted, fmt.Sprintf("%q", opt))
	}
	listExpr := "[]string{" + strings.Join(quoted, ", ") + "}"
	if output != "" {
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "[]string"
		return fmt.Sprintf("%s%s %s %s\n", pad, output, assign, listExpr)
	}
	return fmt.Sprintf("%sslog.Info(\"flow.suggest_next\", \"options\", %s)\n", pad, listExpr)
}

func renderTypedFlowValidate(st *flowRenderState, action flowir.FlowValidate, pad string) string {
	cond := normalizeFlowExpr(action.Condition.Source)
	if cond == "" {
		return ""
	}
	message := action.Message
	if hint := action.Hint; hint != "" {
		message = message + " (hint: " + hint + ")"
	}
	code, status := action.Code, normalizeFlowExpr(action.Status.Source)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif !(%s) {\n", pad, cond))
	b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %q, %q)", status, code, message)))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderTypedFlowExplainError(st *flowRenderState, action flowir.FlowExplainError, pad, sfx string) string {
	errExpr := normalizeFlowExpr(action.Error.Source)
	output, message, hint := action.Output, action.Message, action.Hint
	expMsgV := "_expMsg" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errExpr))
	b.WriteString(fmt.Sprintf("%s\t%s := fmt.Sprintf(\"flow error: %%v\", %s)\n", pad, expMsgV, errExpr))
	if message != "" {
		b.WriteString(fmt.Sprintf("%s\t%s = %q + \": \" + %s\n", pad, expMsgV, message, expMsgV))
	}
	if hint != "" {
		b.WriteString(fmt.Sprintf("%s\t%s = %s + %q\n", pad, expMsgV, expMsgV, " | hint: "+hint))
	}
	if output != "" {
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		b.WriteString(fmt.Sprintf("%s\t%s %s %s\n", pad, output, assign, expMsgV))
	} else {
		b.WriteString(fmt.Sprintf("%s\tslog.Warn(\"flow.explain_error\", \"message\", %s)\n", pad, expMsgV))
	}
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}
