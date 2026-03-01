package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepControlFlow(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "flow.If":
		cond := arg("condition")
		if cond == "" {
			return "", true
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
		return b.String(), true

	case "flow.For":
		each := arg("each")
		as := arg("as")
		if each == "" {
			return "", true
		}
		if as == "" {
			as = "item"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, each))
		b.WriteString(renderFlowSteps(cloneFlowState(st), child("_do"), indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "flow.Block", "tx.Block":
		return renderFlowSteps(st, child("_do"), indent), true

	case "flow.Switch":
		value := arg("value")
		if value == "" {
			return "", true
		}
		cases, _ := step.Args["_cases"].(map[string][]normalizer.FlowStep)
		defaultSteps := child("_default")
		keys := make([]string, 0, len(cases))
		for k := range cases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
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
		return b.String(), true

	case "flow.While":
		cond := arg("condition")
		if cond == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sfor %s {\n", pad, cond))
		b.WriteString(renderFlowSteps(cloneFlowState(st), child("_do"), indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "flow.Checkpoint":
		name := arg("name")
		if name == "" {
			return "", true
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
		return b.String(), true

	case "flow.Resume":
		name := arg("name")
		if name == "" {
			return "", true
		}
		output := arg("output")
		onMissing := child("_onMissing")
		keyLit := fmt.Sprintf("%q", name)
		ckptValV, ckptOKV := "_ckptVal"+sfx, "_ckptOK"+sfx
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
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "any"
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, ckptValV))
		}
		return b.String(), true

	case "flow.Validate":
		cond := arg("condition")
		if cond == "" {
			return "", true
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
		return b.String(), true

	case "flow.Catch":
		catchSteps := child("_do")
		if len(catchSteps) == 0 {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _flowLastError != nil {\n", pad))
		b.WriteString(renderFlowSteps(cloneFlowState(st), catchSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s\t_flowLastError = nil\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "flow.SuggestNext":
		output := arg("output")
		var options []string
		if v, ok := step.Args["options"]; ok {
			switch x := v.(type) {
			case []string:
				options = append(options, x...)
			case string:
				if strings.TrimSpace(x) != "" {
					options = []string{x}
				}
			}
		}
		if len(options) == 0 {
			return "", true
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
			return fmt.Sprintf("%s%s %s %s\n", pad, output, assign, listExpr), true
		}
		return fmt.Sprintf("%sslog.Info(\"flow.suggest_next\", \"options\", %s)\n", pad, listExpr), true

	case "flow.ExplainError":
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
		return b.String(), true
	}

	return "", false
}
