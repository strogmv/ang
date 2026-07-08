package emitter

import (
	"fmt"
	"github.com/strogmv/ang/compiler/flowir"
	"strings"
)

func renderTypedStepString(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "str.StripMarkdown":
		typed, err := typedActionAs[flowir.StringStripMarkdown](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		if output == "" {
			output = input
		}
		tmp := "_sm" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// str.StripMarkdown\n%s%s := strings.TrimSpace(%s)\n", pad, pad, tmp, input))
		b.WriteString(fmt.Sprintf("%sif strings.HasPrefix(%s, \"```\") {\n", pad, tmp))
		b.WriteString(fmt.Sprintf("%s\t%sLines := strings.SplitN(%s, \"\\n\", 2)\n", pad, tmp, tmp))
		b.WriteString(fmt.Sprintf("%s\tif len(%sLines) > 1 {\n%s\t\t%s = %sLines[1]\n", pad, tmp, pad, tmp, tmp))
		b.WriteString(fmt.Sprintf("%s\t\tif _smEnd := strings.LastIndex(%s, \"```\"); _smEnd >= 0 {\n%s\t\t\t%s = strings.TrimSpace(%s[:_smEnd])\n%s\t\t}\n%s\t}\n%s}\n", pad, tmp, pad, tmp, tmp, pad, pad, pad))
		if output == input {
			b.WriteString(fmt.Sprintf("%s%s = strings.TrimSpace(%s)\n", pad, input, tmp))
		} else {
			b.WriteString(renderFlowAssignTarget(st, pad, output, "strings.TrimSpace("+tmp+")", "string"))
		}
		return b.String(), true
	case "str.Format":
		typed, err := typedActionAs[flowir.StringFormat](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		args := make([]string, 0, len(typed.Arguments))
		for _, v := range typed.Arguments {
			args = append(args, normalizeFlowExpr(v.Source))
		}
		call := normalizeFlowExpr(typed.Template.Source)
		if len(args) > 0 {
			call += ", " + strings.Join(args, ", ")
		}
		if strings.Contains(typed.Output, ".") {
			tmp := "_fmt" + sfx
			return fmt.Sprintf("%s%s := fmt.Sprintf(%s)\n%s", pad, tmp, call, renderFlowAssignTarget(st, pad, typed.Output, tmp, "string")), true
		}
		return renderFlowAssignTarget(st, pad, typed.Output, "fmt.Sprintf("+call+")", "string"), true
	case "str.Concat":
		typed, err := typedActionAs[flowir.StringConcat](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		parts := make([]string, 0, len(typed.Parts))
		for _, v := range typed.Parts {
			parts = append(parts, normalizeFlowExpr(v.Source))
		}
		if len(parts) == 0 {
			return renderFlowAssignTarget(st, pad, typed.Output, `""`, "string"), true
		}
		sep := normalizeFlowExpr(typed.Separator.Source)
		if sep == "" {
			for i := range parts {
				parts[i] = "fmt.Sprint(" + parts[i] + ")"
			}
			return renderFlowAssignTarget(st, pad, typed.Output, strings.Join(parts, " + "), "string"), true
		}
		tmp := "_concatParts" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := []string{}\n", pad, tmp))
		for _, p := range parts {
			b.WriteString(fmt.Sprintf("%s%s = append(%s, fmt.Sprint(%s))\n", pad, tmp, tmp, p))
		}
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, fmt.Sprintf("strings.Join(%s, %s)", tmp, sep), "string"))
		return b.String(), true
	}
	return "", false
}
