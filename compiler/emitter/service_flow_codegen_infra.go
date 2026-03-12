package emitter

import "github.com/strogmv/ang-ir/normalizer"

func renderFlowStepInfraLegacy(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	_ = st
	_ = step
	_ = indent
	_ = sfx
	_ = arg
	_ = child
	return ""
}
