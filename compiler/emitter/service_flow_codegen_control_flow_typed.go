package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepControlFlowBasic owns the direct Typed Flow IR dispatch for
// structural control-flow. The AST and text fallbacks below still share the
// established rendering helpers, but their inputs and nested steps come from
// TypedStep rather than reconstructed normalizer.FlowStep arguments.
func renderTypedStepControlFlowBasic(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)
	noRawChildren := func(string) []normalizer.FlowStep { return nil }

	switch step.Name {
	case "flow.If":
		action, err := typedActionAs[flowir.FlowIf](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		arg := func(name string) string {
			if name == "condition" {
				return normalizeFlowExpr(action.Condition.Source)
			}
			return ""
		}
		if out, ok := renderFlowIfAST(st, indent, arg, noRawChildren); ok {
			return out, true
		}
		return renderFlowIfLegacy(st, pad, indent, arg, noRawChildren), true

	case "flow.For":
		action, err := typedActionAs[flowir.FlowFor](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		arg := func(name string) string {
			switch name {
			case "each":
				return normalizeFlowExpr(action.Each.Source)
			case "as":
				return action.As
			default:
				return ""
			}
		}
		if out, ok := renderFlowForAST(st, indent, arg, noRawChildren); ok {
			return out, true
		}
		return renderFlowForLegacy(st, pad, indent, arg, noRawChildren), true

	case "flow.Block", "tx.Block":
		if _, err := typedActionAs[flowir.FlowBlock](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		if out, ok := renderFlowBlockAST(st, indent, noRawChildren); ok {
			return out, true
		}
		return renderFlowBlockLegacy(st, indent, noRawChildren), true

	case "flow.Switch":
		action, err := typedActionAs[flowir.FlowSwitch](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		arg := func(name string) string {
			switch name {
			case "value":
				return normalizeFlowExpr(action.Value.Source)
			case "match":
				return action.Match
			default:
				return ""
			}
		}
		if out, ok := renderFlowSwitchAST(st, nil, indent, arg, nil); ok {
			return out, true
		}
		return renderFlowSwitchLegacy(st, nil, pad, indent, arg, nil), true

	case "flow.While":
		action, err := typedActionAs[flowir.FlowWhile](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		arg := func(name string) string {
			if name == "condition" {
				return normalizeFlowExpr(action.Condition.Source)
			}
			return ""
		}
		if out, ok := renderFlowWhileAST(st, indent, arg, noRawChildren); ok {
			return out, true
		}
		return renderFlowWhileLegacy(st, pad, indent, arg, noRawChildren), true
	}
	return "", false
}

// renderTypedStepControlFlowStateful emits actions that operate on the flow
// checkpoint/history state. Nested paths are read from TypedStep.Children;
// the nil raw collections are compatibility parameters for shared helpers.
func renderTypedStepControlFlowStateful(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Name {
	case "flow.Checkpoint":
		action, err := typedActionAs[flowir.FlowCheckpoint](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		if out, ok := renderFlowCheckpointAST(indent, action); ok {
			return out, true
		}
		return renderFlowCheckpointLegacy(pad, action), true

	case "flow.Resume":
		action, err := typedActionAs[flowir.FlowResume](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowResumeLegacy(st, action, pad, indent, sfx), true

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
		if out, ok := renderFlowValidateAST(st, action, indent); ok {
			return out, true
		}
		return renderFlowValidateLegacy(st, action, pad), true

	case "flow.Catch":
		if _, err := typedActionAs[flowir.FlowCatch](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		if out, ok := renderFlowCatchAST(st, nil, indent); ok {
			return out, true
		}
		return renderFlowCatchLegacy(st, nil, pad, indent), true

	case "flow.Defer":
		if _, err := typedActionAs[flowir.FlowDefer](step); err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		if out, ok := renderFlowDeferAST(st, nil, indent); ok {
			return out, true
		}
		return renderFlowDeferLegacy(st, nil, pad, indent), true

	case "flow.SuggestNext":
		action, err := typedActionAs[flowir.FlowSuggestNext](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		if out, ok := renderFlowSuggestNextAST(st, action, indent); ok {
			return out, true
		}
		return renderFlowSuggestNextLegacy(st, action, pad), true

	case "flow.ExplainError":
		action, err := typedActionAs[flowir.FlowExplainError](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		if out, ok := renderFlowExplainErrorAST(st, action, indent, sfx); ok {
			return out, true
		}
		return renderFlowExplainErrorLegacy(st, action, pad, sfx), true
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
