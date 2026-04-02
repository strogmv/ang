package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
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
		if name == "" {
			return renderInvalidFlowStepConfig(st, pad, "event.Publish", "event.Publish requires name"), true
		}
		if payload == "" {
			return renderInvalidFlowStepConfig(st, pad, "event.Publish", "event.Publish requires renderable payload"), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif s.publisher == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("event.Publish: publisher wiring is not configured")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif err := s.publisher.Publish%s(ctx, %s); err != nil {\n", pad, ExportName(name), payload))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"event.Publish %s: %%w\", err)", ExportName(name))))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "event.EmitIf":
		cond := arg("condition")
		name := arg("name")
		payload := renderEventPayloadExpr(st, step, name, arg)
		if cond == "" || name == "" {
			return renderInvalidFlowStepConfig(st, pad, "event.EmitIf", "event.EmitIf requires condition and name"), true
		}
		if payload == "" {
			return renderInvalidFlowStepConfig(st, pad, "event.EmitIf", "event.EmitIf requires renderable payload"), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s {\n", pad, cond))
		b.WriteString(fmt.Sprintf("%s\tif s.publisher == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `fmt.Errorf("event.EmitIf: publisher wiring is not configured")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif err := s.publisher.Publish%s(ctx, %s); err != nil {\n", pad, ExportName(name), payload))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"event.EmitIf %s: %%w\", err)", ExportName(name))))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "logic.Call":
		funcExpr := arg("func")
		output := arg("output")
		ignoreErr, _ := step.Args["ignoreErr"].(bool)
		ignoreErrReason := strings.TrimSpace(arg("ignoreErrReason"))
		if funcExpr == "" {
			return renderInvalidFlowStepConfig(st, pad, "logic.Call", "logic.Call requires func"), true
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
		if ignoreErr {
			if ignoreErrReason == "" {
				emitFlowWarning(st, step, "FLOW_IGNORE_ERR", "warn", "logic.Call ignores returned error explicitly", "Document intent with ignoreErrReason and use only for deliberate fire-and-forget behavior")
			}
			comment := fmt.Sprintf("%s// explicit ignoreErr=true", pad)
			if ignoreErrReason != "" {
				comment = fmt.Sprintf("%s// explicit ignoreErr=true: %s", pad, ignoreErrReason)
			}
			if output != "" {
				assign := ":="
				if st.declared[output] {
					assign = "="
				}
				st.declared[output] = true
				st.pointers[output] = false
				var b strings.Builder
				b.WriteString(comment + "\n")
				b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output+", _", assign, callStr))
				return b.String(), true
			}
			return fmt.Sprintf("%s\n%s_, _ = %s\n", comment, pad, callStr), true
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
	case "service.Call":
		serviceName := strings.TrimSpace(arg("service"))
		methodName := strings.TrimSpace(arg("method"))
		output := arg("output")
		ignoreErr, _ := step.Args["ignoreErr"].(bool)
		ignoreErrReason := strings.TrimSpace(arg("ignoreErrReason"))
		if serviceName == "" || methodName == "" {
			return renderInvalidFlowStepConfig(st, pad, "service.Call", "service.Call requires service and method"), true
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
			if ignoreErrReason == "" {
				emitFlowWarning(st, step, "FLOW_IGNORE_ERR", "warn", "service.Call ignores returned error explicitly", "Document intent with ignoreErrReason and use only for deliberate fire-and-forget behavior")
			}
			comment := fmt.Sprintf("%s// explicit ignoreErr=true", pad)
			if ignoreErrReason != "" {
				comment = fmt.Sprintf("%s// explicit ignoreErr=true: %s", pad, ignoreErrReason)
			}
			return fmt.Sprintf("%s\n%s_, _ = %s\n", comment, pad, callStr), true
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
