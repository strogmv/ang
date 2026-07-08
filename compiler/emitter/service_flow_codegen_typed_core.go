package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderTypedStepCore(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "str.ReplaceAll":
		typed, err := typedActionAs[flowir.StringReplaceAll](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		expr := fmt.Sprintf("strings.ReplaceAll(%s, %s, %s)", normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Old.Source), normalizeFlowExpr(typed.New.Source))
		return renderFlowAssignTarget(st, pad, typed.Output, expr, "string"), true
	case "str.TrimSpace":
		typed, err := typedActionAs[flowir.StringTrimSpace](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		expr := fmt.Sprintf("strings.TrimSpace(%s)", normalizeFlowExpr(typed.Input.Source))
		return renderFlowAssignTarget(st, pad, typed.Output, expr, "string"), true
	case "cast.ToString":
		typed, err := typedActionAs[flowir.CastToString](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, format := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Format)
		expr := fmt.Sprintf("fmt.Sprint(%s)", input)
		if format != "" {
			expr = fmt.Sprintf("fmt.Sprintf(%s, %s)", format, input)
		}
		return renderFlowAssignTarget(st, pad, typed.Output, expr, "string"), true
	case "template.Render":
		typed, err := typedActionAs[flowir.TemplateRender](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		tmpl, data := normalizeFlowExpr(typed.Template.Source), normalizeFlowExpr(typed.Data.Source)
		tmplVar := "_tmpl" + sfx
		bufVar := "_tmplBuf" + sfx
		errVar := "_tmplErr" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := template.New(\"flow\").Parse(%s)\n", pad, tmplVar, errVar, tmpl))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"template.Render parse: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%svar %s bytes.Buffer\n", pad, bufVar))
		b.WriteString(fmt.Sprintf("%sif %s := %s.Execute(&%s, %s); %s != nil {\n", pad, errVar, tmplVar, bufVar, data, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"template.Render execute: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, bufVar+".String()", "string"))
		return b.String(), true
	case "rand.Code":
		typed, err := typedActionAs[flowir.RandomCode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		baseVar, numberVar := "_codeBase"+sfx, "_codeN"+sfx
		bufferVar, outputVar := "_codeBuf"+sfx, "_codeOut"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := 1\n", pad, baseVar))
		b.WriteString(fmt.Sprintf("%sfor _i := 0; _i < %d; _i++ { %s *= 10 }\n", pad, typed.Length, baseVar))
		b.WriteString(fmt.Sprintf("%s%s := make([]byte, 8)\n", pad, bufferVar))
		b.WriteString(fmt.Sprintf("%sif _, _cErr := cryptorand.Read(%s); _cErr != nil {\n", pad, bufferVar))
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("rand.Code: %w", _cErr)`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := int(binary.BigEndian.Uint64(%s) %% uint64(%s))\n", pad, numberVar, bufferVar, baseVar))
		b.WriteString(fmt.Sprintf("%s%s := fmt.Sprintf(\"%%0%dd\", %s)\n", pad, outputVar, typed.Length, numberVar))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, outputVar, "string"))
		return b.String(), true
	case "rand.Token":
		typed, err := typedActionAs[flowir.RandomToken](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		bytesVar, errVar, outVar := "_rb"+sfx, "_rbErr"+sfx, "_tokenOut"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := make([]byte, %d)\n", pad, bytesVar, typed.Bytes))
		b.WriteString(fmt.Sprintf("%s_, %s := cryptorand.Read(%s)\n", pad, errVar, bytesVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("rand.Token: %w", `+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n%s%s := hex.EncodeToString(%s)\n", pad, pad, outVar, bytesVar))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, outVar, "string"))
		return b.String(), true
	case "stream.Emit":
		typed, err := typedActionAs[flowir.StreamEmit](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		data := normalizeFlowExpr(typed.Data.Source)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sselect {\n", pad))
		b.WriteString(fmt.Sprintf("%scase <-ctx.Done():\n", pad+"\t"))
		b.WriteString(errReturn(st, pad+"\t\t", "ctx.Err()"))
		b.WriteString(fmt.Sprintf("%scase chunks <- fmt.Sprint(%s):\n", pad+"\t", data))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "session.Get":
		typed, err := typedActionAs[flowir.SessionGet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		st.types[typed.Output] = "string"
		return fmt.Sprintf("%s%s %s reqctx.SessionID(ctx)\n", pad, typed.Output, assign), true

	case "event.Publish":
		typed, err := typedActionAs[flowir.EventPublish](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		payload := renderTypedEventPayloadExpr(st, typed.Event, typed.Payload, typed.PayloadMap)
		if payload == "" {
			return renderInvalidFlowStepConfig(st, pad, step.Name, "event.Publish requires renderable payload"), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif s.publisher == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("event.Publish: publisher wiring is not configured")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif err := s.publisher.Publish%s(ctx, %s); err != nil {\n", pad, ExportName(typed.Event), payload))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"event.Publish %s: %%w\", err)", ExportName(typed.Event))))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "event.EmitIf":
		typed, err := typedActionAs[flowir.EventEmitIf](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		condition := normalizeFlowExpr(typed.Condition.Source)
		payload := renderTypedEventPayloadExpr(st, typed.Event, typed.Payload, typed.PayloadMap)
		if payload == "" {
			return renderInvalidFlowStepConfig(st, pad, step.Name, "event.EmitIf requires renderable payload"), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s {\n", pad, condition))
		b.WriteString(fmt.Sprintf("%s\tif s.publisher == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `fmt.Errorf("event.EmitIf: publisher wiring is not configured")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif err := s.publisher.Publish%s(ctx, %s); err != nil {\n", pad, ExportName(typed.Event), payload))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"event.EmitIf %s: %%w\", err)", ExportName(typed.Event))))
		b.WriteString(fmt.Sprintf("%s\t}\n%s}\n", pad, pad))
		return b.String(), true
	}
	return "", false
}

func renderTypedStepCall(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "flow.Call":
		typed, err := typedActionAs[flowir.FlowCall](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowCall(st, typed, pad), true

	case "logic.Call":
		typed, err := typedActionAs[flowir.LogicCall](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		args := make([]string, 0, len(typed.Arguments))
		for _, expression := range typed.Arguments {
			args = append(args, expression.Source)
		}
		call := typed.Function.Source + "(" + strings.Join(args, ", ") + ")"
		return renderTypedCallResult(st, step.Name, typed.Output, typed.IgnoreError, typed.IgnoreErrReason, call, pad), true

	case "service.Call":
		typed, err := typedActionAs[flowir.ServiceCall](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		args := make([]string, 0, len(typed.Arguments)+1)
		for _, expression := range typed.Arguments {
			args = append(args, expression.Source)
		}
		if len(args) == 0 || strings.TrimSpace(args[0]) != "ctx" {
			args = append([]string{"ctx"}, args...)
		}
		call := fmt.Sprintf("s.%sService.%s(%s)", ExportName(typed.Service), ExportName(typed.Method), strings.Join(args, ", "))
		output := typed.Output
		if typed.IgnoreError {
			// Preserve service.Call's legacy fire-and-forget contract.
			output = ""
		}
		return renderTypedCallResult(st, step.Name, output, typed.IgnoreError, typed.IgnoreErrReason, call, pad), true
	}
	return "", false
}

func renderTypedCallResult(st *flowRenderState, action, output string, ignoreErr bool, ignoreReason, call, pad string) string {
	if ignoreErr {
		if ignoreReason == "" {
			emitFlowWarning(st, "FLOW_IGNORE_ERR", "warn", action+" ignores returned error explicitly", "Document intent with ignoreErrReason and use only for deliberate fire-and-forget behavior")
		}
		comment := pad + "// explicit ignoreErr=true"
		if ignoreReason != "" {
			comment += ": " + ignoreReason
		}
		if output == "" {
			return fmt.Sprintf("%s\n%s_, _ = %s\n", comment, pad, call)
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		return fmt.Sprintf("%s\n%s%s %s %s\n", comment, pad, output+", _", assign, call)
	}
	if output == "" {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _, err := %s; err != nil {\n", pad, call))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()
	}
	assign := ":="
	if st.declared[output] {
		assign = "="
	}
	st.declared[output] = true
	st.pointers[output] = false
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output+", err", assign, call))
	b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
	b.WriteString(errReturn(st, pad+"\t", "err"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}
