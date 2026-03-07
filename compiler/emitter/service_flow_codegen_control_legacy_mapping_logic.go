package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepControlLegacyMappingLogic(st *flowRenderState, step normalizer.FlowStep, pad string, arg func(string) string) (string, bool) {
	switch step.Action {
	case "mapping.Map":
		from := arg("from")
		to := arg("to")
		input := arg("input")
		output := arg("output")
		entity := arg("entity")
		if from != "" && to != "" {
			toRef := to
			if strings.Contains(to, ".") {
				parts := strings.Split(to, ".")
				parts[len(parts)-1] = ExportName(parts[len(parts)-1])
				toRef = strings.Join(parts, ".")
			}
			if entity != "" && !st.declared[to] {
				st.declared[to] = true
				st.pointers[to] = false
				var b strings.Builder
				b.WriteString(fmt.Sprintf("%svar %s domain.%s\n", pad, to, entity))
				b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, to, from))
				b.WriteString(errReturn(st, pad+"\t", "err"))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
				return b.String(), true
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, toRef, from))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		if input != "" && output != "" {
			outputRef := output
			if strings.Contains(output, ".") {
				parts := strings.Split(output, ".")
				parts[len(parts)-1] = ExportName(parts[len(parts)-1])
				outputRef = strings.Join(parts, ".")
			}
			if entity != "" && !st.declared[output] {
				st.declared[output] = true
				st.pointers[output] = false
				var b strings.Builder
				b.WriteString(fmt.Sprintf("%svar %s domain.%s\n", pad, output, entity))
				b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, output, input))
				b.WriteString(errReturn(st, pad+"\t", "err"))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
				return b.String(), true
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, outputRef, input))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		if output != "" && entity != "" && !st.declared[output] {
			st.declared[output] = true
			st.pointers[output] = false
			return fmt.Sprintf("%svar %s domain.%s\n", pad, output, entity), true
		}
		return "", true

	case "event.Publish":
		name := arg("name")
		payload := renderEventPayloadExpr(st, step, name, arg)
		if name == "" || payload == "" {
			return "", true
		}
		return fmt.Sprintf("%sif s.publisher != nil {\n%s\t_ = s.publisher.Publish%s(ctx, %s)\n%s}\n",
			pad, pad, ExportName(name), payload, pad), true

	case "logic.Call":
		funcExpr := arg("func")
		output := arg("output")
		if funcExpr == "" {
			return "", true
		}
		var callArgs []string
		if v, ok := step.Args["args"]; ok {
			switch x := v.(type) {
			case []string:
				callArgs = x
			case string:
				callArgs = []string{x}
			}
		}
		callStr := funcExpr + "(" + strings.Join(callArgs, ", ") + ")"
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output+", err", assign, callStr))
			b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _, err := %s; err != nil {\n", pad, callStr))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	case "service.Call":
		serviceName := strings.TrimSpace(arg("service"))
		methodName := strings.TrimSpace(arg("method"))
		output := arg("output")
		ignoreErr, _ := step.Args["ignoreErr"].(bool)
		if serviceName == "" || methodName == "" {
			return "", true
		}
		var callArgs []string
		if v, ok := step.Args["args"]; ok {
			switch x := v.(type) {
			case []string:
				callArgs = append(callArgs, x...)
			case string:
				callArgs = append(callArgs, x)
			}
		}
		if len(callArgs) == 0 || strings.TrimSpace(callArgs[0]) != "ctx" {
			callArgs = append([]string{"ctx"}, callArgs...)
		}
		callStr := fmt.Sprintf("s.%sService.%s(%s)", ExportName(serviceName), ExportName(methodName), strings.Join(callArgs, ", "))
		if ignoreErr {
			return fmt.Sprintf("%s_, _ = %s\n", pad, callStr), true
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output+", err", assign, callStr))
			b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _, err := %s; err != nil {\n", pad, callStr))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}

	return "", false
}
