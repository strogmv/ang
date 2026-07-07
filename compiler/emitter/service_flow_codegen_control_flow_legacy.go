package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderFlowBlockLegacy(st *flowRenderState, indent int, child func(string) []normalizer.FlowStep) string {
	return renderFlowSteps(st, child("_do"), indent)
}

func renderFlowIfLegacy(st *flowRenderState, pad string, indent int, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	cond := arg("condition")
	if cond == "" {
		return ""
	}
	thenSteps := child("_then")
	elseSteps := child("_else")
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif %s {\n", pad, cond))
	b.WriteString(renderFlowSteps(cloneFlowState(st), thenSteps, indent+1))
	b.WriteString(fmt.Sprintf("%s}", pad))
	if len(elseSteps) > 0 {
		b.WriteString(" else {\n")
		b.WriteString(renderFlowSteps(cloneFlowState(st), elseSteps, indent+1))
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
	b.WriteString(renderFlowSteps(cloneFlowState(st), child("_do"), indent+1))
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
	keys := make([]string, 0, len(cases))
	for k := range cases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	if matchMode == "exact" {
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, value))
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("%scase %q:\n", pad, k))
			b.WriteString(renderFlowSteps(cloneFlowState(st), cases[k], indent+1))
		}
		if len(defaultSteps) > 0 {
			b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
			b.WriteString(renderFlowSteps(cloneFlowState(st), defaultSteps, indent+1))
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
		b.WriteString(renderFlowSteps(cloneFlowState(st), cases[k], indent+1))
	}
	if len(defaultSteps) > 0 {
		if len(keys) > 0 {
			b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		} else {
			b.WriteString(fmt.Sprintf("%s{\n", pad))
		}
		b.WriteString(renderFlowSteps(cloneFlowState(st), defaultSteps, indent+1))
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
	b.WriteString(renderFlowSteps(cloneFlowState(st), child("_do"), indent+1))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowCheckpointLegacy(pad string, arg func(string) string) string {
	name := arg("name")
	if name == "" {
		return ""
	}
	data := arg("data")
	if data == "" {
		data = "map[string]any{\"resp\": resp}"
	}
	keyLit := fmt.Sprintf("%q", name)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif _flowCheckpoints == nil {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_flowCheckpoints = make(map[string]any)\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s_flowCheckpoints[%s] = %s\n", pad, keyLit, data))
	return b.String()
}

func renderFlowResumeLegacy(st *flowRenderState, pad string, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	name := arg("name")
	if name == "" {
		return ""
	}
	output := arg("output")
	into := arg("into")
	onMissing := child("_onMissing")
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
	if len(onMissing) > 0 {
		b.WriteString(renderFlowSteps(cloneFlowState(st), onMissing, indent+1))
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

func renderFlowCatchLegacy(st *flowRenderState, pad string, indent int, child func(string) []normalizer.FlowStep) string {
	catchSteps := child("_do")
	if len(catchSteps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif _flowLastError != nil {\n", pad))
	b.WriteString(renderFlowSteps(cloneFlowState(st), catchSteps, indent+1))
	b.WriteString(fmt.Sprintf("%s\t_flowLastError = nil\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowSuggestNextLegacy(st *flowRenderState, step normalizer.FlowStep, pad string, arg func(string) string) string {
	output := arg("output")
	options := flowSuggestNextOptions(step)
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

func renderFlowValidateLegacy(st *flowRenderState, pad string, arg func(string) string) string {
	cond := arg("condition")
	if cond == "" {
		return ""
	}
	message := arg("message")
	if message == "" {
		message = arg("throw")
	}
	if message == "" {
		message = "validation failed"
	}
	if hint := arg("hint"); hint != "" {
		message = message + " (hint: " + hint + ")"
	}
	code := arg("code")
	if code == "" {
		code = "VALIDATION_FAILED"
	}
	status := arg("status")
	if status == "" {
		status = "http.StatusBadRequest"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif !(%s) {\n", pad, cond))
	b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %q, %q)", status, code, message)))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowExplainErrorLegacy(st *flowRenderState, pad, sfx string, arg func(string) string) string {
	errExpr := arg("error")
	if errExpr == "" {
		errExpr = "_flowLastError"
	}
	output := arg("output")
	message := arg("message")
	hint := arg("hint")
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
