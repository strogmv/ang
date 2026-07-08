package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderTypedStepJSON(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	if step.Name == "json.Parse" {
		typed, err := typedActionAs[flowir.JSONParse](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(typed.Input.Source)
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output], st.pointers[typed.Output], st.types[typed.Output] = true, false, typed.Into
		var b strings.Builder
		if assign == ":=" {
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, typed.Output, typed.Into))
		}
		b.WriteString(fmt.Sprintf("%sif _jErr := json.Unmarshal([]byte(%s), &%s); _jErr != nil {\n", pad, input, typed.Output))
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("json: %w", _jErr)`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}
	if step.Name == "json.Marshal" || step.Name == "json.Stringify" {
		typed, err := typedActionAs[flowir.JSONMarshal](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output], st.pointers[typed.Output], st.types[typed.Output] = true, false, "string"
		bytesVar, errVar, label := "_jb"+sfx, "_jErr"+sfx, "json"
		if step.Name == "json.Stringify" {
			bytesVar, errVar, label = "_jsb"+sfx, "_jsErr"+sfx, "json.Stringify"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := json.Marshal(%s)\n", pad, bytesVar, errVar, normalizeFlowExpr(typed.Input.Source)))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"%s: %%w\", %s)", label, errVar)))
		b.WriteString(fmt.Sprintf("%s}\n%s%s %s string(%s)\n", pad, pad, typed.Output, assign, bytesVar))
		return b.String(), true
	}
	return "", false
}
