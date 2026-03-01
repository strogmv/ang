package emitter

import "github.com/strogmv/ang/compiler/normalizer"

// renderFlowStepControlLegacy is a thin compatibility dispatch:
// keep the actively maintained legacy handlers in a compact core, then
// fallback to the historical monolith for any remaining edge action.
func renderFlowStepControlLegacy(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	if out, ok := renderFlowStepControlLegacyCore(st, step, indent, sfx, arg); ok {
		return out
	}
	return renderFlowStepControlLegacyDeprecated(st, step, indent, sfx, arg, child)
}
