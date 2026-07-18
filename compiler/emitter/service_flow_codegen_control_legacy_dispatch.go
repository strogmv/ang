package emitter

import "github.com/strogmv/ang-ir/normalizer"

// renderFlowStepControlLegacy remains only for the historical raw-step
// compatibility path. Typed actions are dispatched by renderTypedStepControl.
func renderFlowStepControlLegacy(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	return renderFlowStepControlLegacyDeprecated(st, step, indent, sfx, arg, child)
}
