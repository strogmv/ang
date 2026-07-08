package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowStepControlLegacyMappingLogic(st *flowRenderState, step normalizer.FlowStep, pad string, arg func(string) string) (string, bool) {
	switch step.Action {
	case "mapping.Map":
		typed, decodeErr := decodeCurrentActionAs[flowir.MappingMap](st, step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, "mapping.Map", decodeErr.Error()), true
		}
		from := typed.Input.Source
		to := typed.Output
		input := typed.Input.Source
		output := typed.Output
		entity := typed.Entity
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
		typed, decodeErr := decodeCurrentActionAs[flowir.EventPublish](st, step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, "event.Publish", decodeErr.Error()), true
		}
		name := typed.Event
		payload := renderTypedEventPayloadExpr(st, name, typed.Payload, typed.PayloadMap)
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
		typed, err := decodeCurrentActionAs[flowir.EventEmitIf](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		cond, name := normalizeFlowExpr(typed.Condition.Source), typed.Event
		payload := renderTypedEventPayloadExpr(st, name, typed.Payload, typed.PayloadMap)
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
		call, callErr := decodeCurrentActionAs[flowir.LogicCall](st, step)
		if callErr != nil {
			return renderInvalidFlowStepConfig(st, pad, "logic.Call", callErr.Error()), true
		}
		if call.Function.Source == "" {
			return renderInvalidFlowStepConfig(st, pad, "logic.Call", "logic.Call requires func"), true
		}
		callArgs := make([]string, 0, len(call.Arguments))
		for _, expression := range call.Arguments {
			callArgs = append(callArgs, expression.Source)
		}
		callStr := call.Function.Source + "(" + strings.Join(callArgs, ", ") + ")"
		if call.IgnoreError {
			if call.IgnoreErrReason == "" {
				emitFlowWarning(st, step, "FLOW_IGNORE_ERR", "warn", "logic.Call ignores returned error explicitly", "Document intent with ignoreErrReason and use only for deliberate fire-and-forget behavior")
			}
			comment := fmt.Sprintf("%s// explicit ignoreErr=true", pad)
			if call.IgnoreErrReason != "" {
				comment = fmt.Sprintf("%s// explicit ignoreErr=true: %s", pad, call.IgnoreErrReason)
			}
			if call.Output != "" {
				assign := ":="
				if st.declared[call.Output] {
					assign = "="
				}
				st.declared[call.Output] = true
				st.pointers[call.Output] = false
				var b strings.Builder
				b.WriteString(comment + "\n")
				b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, call.Output+", _", assign, callStr))
				return b.String(), true
			}
			return fmt.Sprintf("%s\n%s_, _ = %s\n", comment, pad, callStr), true
		}
		if call.Output != "" {
			assign := ":="
			if st.declared[call.Output] {
				assign = "="
			}
			st.declared[call.Output] = true
			st.pointers[call.Output] = false
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, call.Output+", err", assign, callStr))
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
		if strings.TrimSpace(arg("service")) == "" || strings.TrimSpace(arg("method")) == "" {
			return renderInvalidFlowStepConfig(st, pad, "service.Call", "service.Call requires service and method"), true
		}
		call, callErr := decodeCurrentActionAs[flowir.ServiceCall](st, step)
		if callErr != nil {
			return renderInvalidFlowStepConfig(st, pad, "service.Call", callErr.Error()), true
		}
		callArgs := make([]string, 0, len(call.Arguments)+1)
		for _, expression := range call.Arguments {
			callArgs = append(callArgs, expression.Source)
		}
		if len(callArgs) == 0 || strings.TrimSpace(callArgs[0]) != "ctx" {
			callArgs = append([]string{"ctx"}, callArgs...)
		}
		callStr := fmt.Sprintf("s.%sService.%s(%s)", ExportName(call.Service), ExportName(call.Method), strings.Join(callArgs, ", "))
		if call.IgnoreError {
			if call.IgnoreErrReason == "" {
				emitFlowWarning(st, step, "FLOW_IGNORE_ERR", "warn", "service.Call ignores returned error explicitly", "Document intent with ignoreErrReason and use only for deliberate fire-and-forget behavior")
			}
			comment := fmt.Sprintf("%s// explicit ignoreErr=true", pad)
			if call.IgnoreErrReason != "" {
				comment = fmt.Sprintf("%s// explicit ignoreErr=true: %s", pad, call.IgnoreErrReason)
			}
			return fmt.Sprintf("%s\n%s_, _ = %s\n", comment, pad, callStr), true
		}
		if call.Output != "" {
			assign := ":="
			if st.declared[call.Output] {
				assign = "="
			}
			st.declared[call.Output] = true
			st.pointers[call.Output] = false
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, call.Output+", err", assign, callStr))
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
