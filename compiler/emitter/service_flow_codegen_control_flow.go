package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowTag(st *flowRenderState, action flowir.FlowTag, indent int) string {
	pad := strings.Repeat("\t", indent)
	name := normalizeFlowExpr(action.Name.Source)
	value := normalizeFlowExpr(action.Value.Source)
	if name == "" {
		return renderInvalidFlowStepConfig(st, pad, "flow.Tag", "flow.Tag requires name")
	}

	if value != "" {
		return fmt.Sprintf("%sslog.Info(\"flow.tag\", \"name\", %s, \"value\", %s)\n", pad, name, value)
	}
	return fmt.Sprintf("%sslog.Info(\"flow.tag\", \"name\", %s)\n", pad, name)
}

func renderFlowCall(st *flowRenderState, action flowir.FlowCall, pad string) string {
	opRaw := action.Operation
	if opRaw == "" {
		return renderInvalidFlowStepConfig(st, pad, "flow.Call", "flow.Call requires op")
	}
	output, ignoreErr, ignoreErrReason := action.Output, action.IgnoreError, action.IgnoreErrReason

	serviceName := ""
	methodName := opRaw
	if parts := strings.SplitN(opRaw, ".", 2); len(parts) == 2 {
		serviceName = strings.TrimSpace(parts[0])
		methodName = strings.TrimSpace(parts[1])
	}
	if methodName == "" {
		return renderInvalidFlowStepConfig(st, pad, "flow.Call", "flow.Call requires valid op method name")
	}
	methodExport := ExportName(methodName)

	reqExpr := fmt.Sprintf("port.%sRequest{}", methodExport)
	argMap := action.Arguments
	if len(argMap) > 0 {
		keys := make([]string, 0, len(argMap))
		for k := range argMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fields := make([]string, 0, len(keys))
		for _, k := range keys {
			expr := normalizeFlowExpr(argMap[k].Source)
			if expr == "" {
				continue
			}
			fields = append(fields, fmt.Sprintf("%s: %s", ExportName(k), expr))
		}
		if len(fields) > 0 {
			reqExpr = fmt.Sprintf("port.%sRequest{%s}", methodExport, strings.Join(fields, ", "))
		}
	}

	callStr := fmt.Sprintf("s.%s(ctx, %s)", methodExport, reqExpr)
	if strings.TrimSpace(serviceName) != "" && !strings.EqualFold(strings.TrimSpace(serviceName), strings.TrimSpace(st.serviceName)) {
		callStr = fmt.Sprintf("s.%sService.%s(ctx, %s)", ExportName(serviceName), methodExport, reqExpr)
	}

	if ignoreErr {
		if ignoreErrReason == "" {
			emitFlowWarning(st, "FLOW_IGNORE_ERR", "warn", "flow.Call ignores returned error explicitly", "Document intent with ignoreErrReason and use only for deliberate fire-and-forget behavior")
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
			st.types[output] = "port." + methodExport + "Response"
			return fmt.Sprintf("%s\n%s%s %s %s\n", comment, pad, output+", _", assign, callStr)
		}
		return fmt.Sprintf("%s\n%s_, _ = %s\n", comment, pad, callStr)
	}

	if output != "" {
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "port." + methodExport + "Response"
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output+", err", assign, callStr))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif _, err := %s; err != nil {\n", pad, callStr))
	b.WriteString(errReturn(st, pad+"\t", "err"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderTypedFlowDefer(st *flowRenderState, step flowir.TypedStep, pad string, indent int) string {
	deferSteps := step.Children["_do"]
	if len(deferSteps) == 0 {
		return ""
	}
	predecl := flowTypedDeferPredeclaredStringVars(st, deferSteps)
	for _, name := range predecl {
		st.declared[name] = true
		st.pointers[name] = false
		st.types[name] = "string"
	}

	var b strings.Builder
	for _, name := range predecl {
		b.WriteString(fmt.Sprintf("%svar %s string\n", pad, name))
	}
	b.WriteString(fmt.Sprintf("%sdefer func() {\n", pad))
	deferState := cloneFlowState(st)
	deferState.concurrMode = "race"
	b.WriteString(renderTypedFlowSteps(deferState, deferSteps, indent+1))
	b.WriteString(fmt.Sprintf("%s}()\n", pad))
	return b.String()
}

func flowTypedDeferPredeclaredStringVars(st *flowRenderState, steps []flowir.TypedStep) []string {
	set := map[string]struct{}{}
	var walk func([]flowir.TypedStep)
	walk = func(items []flowir.TypedStep) {
		for _, s := range items {
			if action, ok := s.Action.(flowir.FSRemove); ok {
				if ident, ok := flowSimpleIdent(normalizeFlowExpr(action.Path.Source)); ok && !st.declared[ident] {
					set[ident] = struct{}{}
				}
			}
			for _, nested := range s.Children {
				walk(nested)
			}
			for _, nested := range s.Branches {
				walk(nested)
			}
		}
	}
	walk(steps)
	if len(set) == 0 {
		return nil
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func flowSimpleIdent(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	for i, r := range s {
		if i == 0 {
			if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return "", false
			}
			continue
		}
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return "", false
		}
	}
	return s, true
}
