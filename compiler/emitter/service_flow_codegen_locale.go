package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderTypedStepLocale(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Name {
	case "locale.Resolve":
		typed, err := typedActionAs[flowir.LocaleResolve](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		output, defaultLocale, sources := typed.Output, normalizeFlowExpr(typed.Default.Source), typed.Sources

		declareOutput := !st.declared[output]
		if declareOutput {
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "string"
		}

		var b strings.Builder
		assign := "="
		if declareOutput {
			assign = ":="
		}
		b.WriteString(fmt.Sprintf("%s%s %s \"\"\n", pad, output, assign))

		// Try each source expression in order
		if sources != "" {
			for _, src := range strings.Split(sources, ",") {
				src = strings.TrimSpace(src)
				if src == "" {
					continue
				}
				b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, output))
				b.WriteString(fmt.Sprintf("%s\tif _v := fmt.Sprint(%s); _v != \"\" && _v != \"<nil>\" {\n", pad, src))
				b.WriteString(fmt.Sprintf("%s\t\t%s = _v\n", pad, output))
				b.WriteString(fmt.Sprintf("%s\t}\n", pad))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
			}
		}

		// Fallback to reqctx.Locale(ctx)
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, output))
		b.WriteString(fmt.Sprintf("%s\t%s = reqctx.Locale(ctx)\n", pad, output))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		// Final default
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, output))
		b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, output, defaultLocale))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		return b.String(), true
	}
	return "", false
}
