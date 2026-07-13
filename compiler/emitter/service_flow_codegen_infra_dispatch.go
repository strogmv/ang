package emitter

import "github.com/strogmv/ang-ir/normalizer"

func renderFlowStepInfra(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	if out, ok := renderFlowStepInfraClaude(st, step, indent, sfx, arg, child); ok {
		return out
	}
	if out, ok := renderFlowStepMetaPlan(st, step, indent, sfx, arg, child); ok {
		return out
	}
	if out, ok := renderFlowStepInfraOpenAI(st, step, indent, sfx, arg, child); ok {
		return out
	}
	if out, ok := renderFlowStepLocale(st, step, indent, sfx, arg, child); ok {
		return out
	}
	return renderFlowStepInfraLegacy(st, step, indent, sfx, arg, child)
}
