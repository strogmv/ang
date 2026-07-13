package emitter

import "github.com/strogmv/ang-ir/normalizer"

func renderFlowStepInfra(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	return renderFlowStepInfraLegacy(st, step, indent, sfx, arg, child)
}
