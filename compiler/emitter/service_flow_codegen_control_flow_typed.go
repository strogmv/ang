package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepControlFlowBasic owns the direct Typed Flow IR dispatch for
// structural control-flow. It reads nested control paths solely from
// TypedStep.Children and TypedStep.Branches.
func renderTypedStepControlFlowBasic(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Name {
	case "flow.If":
		action, err := typedActionAs[flowir.FlowIf](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowIf(st, step, indent, normalizeFlowExpr(action.Condition.Source)), true

	case "flow.For":
		action, err := typedActionAs[flowir.FlowFor](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowFor(st, step, indent, normalizeFlowExpr(action.Each.Source), action.As), true

	case "flow.Block", "tx.Block":
		if _, err := typedActionAs[flowir.FlowBlock](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowSteps(cloneFlowState(st), step.Children["_do"], indent), true

	case "flow.Switch":
		action, err := typedActionAs[flowir.FlowSwitch](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowSwitch(st, step, indent, normalizeFlowExpr(action.Value.Source), action.Match), true

	case "flow.While":
		action, err := typedActionAs[flowir.FlowWhile](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowWhile(st, step, indent, normalizeFlowExpr(action.Condition.Source)), true
	}
	return "", false
}

func renderTypedFlowIf(st *flowRenderState, step flowir.TypedStep, indent int, condition string) string {
	if condition == "" {
		return ""
	}
	pad := strings.Repeat("\t", indent)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif %s {\n", pad, condition))
	b.WriteString(renderTypedFlowSteps(cloneFlowState(st), step.Children["_then"], indent+1))
	b.WriteString(fmt.Sprintf("%s}", pad))
	if elseSteps := step.Children["_else"]; len(elseSteps) > 0 {
		b.WriteString(" else {\n")
		b.WriteString(renderTypedFlowSteps(cloneFlowState(st), elseSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}", pad))
	}
	b.WriteString("\n")
	return b.String()
}

func renderTypedFlowFor(st *flowRenderState, step flowir.TypedStep, indent int, each, as string) string {
	if each == "" {
		return ""
	}
	if as == "" {
		as = "item"
	}
	pad := strings.Repeat("\t", indent)
	return fmt.Sprintf("%sfor _, %s := range %s {\n%s%s}\n", pad, as, each,
		renderTypedFlowSteps(cloneFlowState(st), step.Children["_do"], indent+1), pad)
}

func renderTypedFlowSwitch(st *flowRenderState, step flowir.TypedStep, indent int, value, matchMode string) string {
	if value == "" {
		return ""
	}
	pad := strings.Repeat("\t", indent)
	matchMode = strings.ToLower(strings.TrimSpace(matchMode))
	if matchMode == "" {
		matchMode = "exact"
	}
	keys := make([]string, 0, len(step.Branches))
	for key := range step.Branches {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	if matchMode == "exact" {
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, value))
		for _, key := range keys {
			b.WriteString(fmt.Sprintf("%scase %q:\n", pad, key))
			b.WriteString(renderTypedFlowSteps(cloneFlowState(st), step.Branches[key], indent+1))
		}
		if defaultSteps := step.Children["_default"]; len(defaultSteps) > 0 {
			b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
			b.WriteString(renderTypedFlowSteps(cloneFlowState(st), defaultSteps, indent+1))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()
	}
	selector := "_switchValue"
	b.WriteString(fmt.Sprintf("%s%s := strings.TrimSpace(fmt.Sprint(%s))\n", pad, selector, value))
	for i, key := range keys {
		condition := fmt.Sprintf("%s == %q", selector, key)
		switch matchMode {
		case "prefix":
			condition = fmt.Sprintf("strings.HasPrefix(%s, %q)", selector, key)
		case "suffix":
			condition = fmt.Sprintf("strings.HasSuffix(%s, %q)", selector, key)
		case "contains":
			condition = fmt.Sprintf("strings.Contains(%s, %q)", selector, key)
		case "glob":
			condition = fmt.Sprintf("_switchMatch, _ := path.Match(%q, %s); _switchMatch", key, selector)
		}
		if i == 0 {
			b.WriteString(fmt.Sprintf("%sif %s {\n", pad, condition))
		} else {
			b.WriteString(fmt.Sprintf("%s} else if %s {\n", pad, condition))
		}
		b.WriteString(renderTypedFlowSteps(cloneFlowState(st), step.Branches[key], indent+1))
	}
	if defaultSteps := step.Children["_default"]; len(defaultSteps) > 0 {
		if len(keys) > 0 {
			b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		} else {
			b.WriteString(fmt.Sprintf("%s{\n", pad))
		}
		b.WriteString(renderTypedFlowSteps(cloneFlowState(st), defaultSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	} else if len(keys) > 0 {
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	return b.String()
}

func renderTypedFlowWhile(st *flowRenderState, step flowir.TypedStep, indent int, condition string) string {
	if condition == "" {
		return ""
	}
	pad := strings.Repeat("\t", indent)
	return fmt.Sprintf("%sfor %s {\n%s%s}\n", pad, condition,
		renderTypedFlowSteps(cloneFlowState(st), step.Children["_do"], indent+1), pad)
}

// renderTypedStepControlFlowStateful emits actions that operate on the flow
// checkpoint/history state. Nested paths are read from TypedStep.Children.
func renderTypedStepControlFlowStateful(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Name {
	case "flow.Checkpoint":
		action, err := typedActionAs[flowir.FlowCheckpoint](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowCheckpoint(pad, action), true

	case "flow.Resume":
		action, err := typedActionAs[flowir.FlowResume](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowResume(st, step, action, pad, indent, sfx), true

	case "flow.RecordEvent":
		action, err := typedActionAs[flowir.FlowRecordEvent](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowRecordEvent(st, action, indent, sfx), true

	case "flow.History.Get":
		action, err := typedActionAs[flowir.FlowHistoryGet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowHistoryGet(st, action, indent, sfx), true

	case "flow.Replay":
		action, err := typedActionAs[flowir.FlowReplay](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowReplay(st, action, indent, sfx), true

	case "flow.Validate":
		action, err := typedActionAs[flowir.FlowValidate](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowValidate(st, action, pad), true

	case "flow.Catch":
		if _, err := typedActionAs[flowir.FlowCatch](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowCatch(st, step, pad, indent), true

	case "flow.Defer":
		if _, err := typedActionAs[flowir.FlowDefer](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowDefer(st, step, pad, indent), true

	case "flow.SuggestNext":
		action, err := typedActionAs[flowir.FlowSuggestNext](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowSuggestNext(st, action, pad), true

	case "flow.ExplainError":
		action, err := typedActionAs[flowir.FlowExplainError](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedFlowExplainError(st, action, pad, sfx), true
	}
	return "", false
}

// renderTypedStepControlParallel validates the typed action before entering
// the shared concurrent renderer. That renderer reads branches through
// st.currentTyped, so nil is deliberately supplied for the raw compatibility
// map and no normalizer branch tree is reconstructed in production.
func renderTypedStepControlParallel(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "flow.Parallel":
		if _, err := typedActionAs[flowir.FlowParallel](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowParallel(st, nil, indent, sfx), true
	case "flow.Join":
		if _, err := typedActionAs[flowir.FlowJoin](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowJoin(st, nil, indent, sfx), true
	case "flow.Race":
		if _, err := typedActionAs[flowir.FlowRace](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowRace(st, nil, indent, sfx), true
	}
	return "", false
}

// renderTypedStepControlScheduling covers simple timing and terminal actions.
// These actions have no raw compatibility input: their behaviour is fully
// defined by the decoded flowir action and, for flow.Cron, typed children.
func renderTypedStepControlScheduling(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "flow.Delay":
		action, err := typedActionAs[flowir.FlowDelay](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowDelay(st, action, indent, sfx), true
	case "flow.Schedule":
		action, err := typedActionAs[flowir.FlowSchedule](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowSchedule(st, action, indent, sfx), true
	case "flow.Cron":
		action, err := typedActionAs[flowir.FlowCron](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowCron(st, action, indent, sfx), true
	case "flow.Tag":
		action, err := typedActionAs[flowir.FlowTag](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowTag(st, action, indent), true
	case "flow.Return":
		action, err := typedActionAs[flowir.FlowReturn](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		setField := normalizeFlowExpr(action.Set.Source)
		setValue := normalizeFlowExpr(action.Value.Source)
		if setField == "" || setValue == "" {
			return returnSuccess(st, pad), true
		}
		return fmt.Sprintf("%s%s = %s\n%s", pad, setField, setValue, returnSuccess(st, pad)), true
	}
	return "", false
}
