package emitter

import "github.com/strogmv/ang-ir/normalizer"

func renderFlowStepControl(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	if out, ok := renderFlowStepSaga(st, step, indent, sfx, arg, child); ok {
		return out
	}
	if out, ok := renderFlowStepControlFlow(st, step, indent, sfx, arg, child); ok {
		return out
	}
	return renderFlowStepControlLegacy(st, step, indent, sfx, arg, child)
}
