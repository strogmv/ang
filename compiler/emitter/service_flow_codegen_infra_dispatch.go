package emitter

import "github.com/strogmv/ang/compiler/normalizer"

func renderFlowStepInfra(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	if out, ok := renderFlowStepInfraCacheMailStorage(st, step, indent, sfx, arg, child); ok {
		return out
	}
	if out, ok := renderFlowStepInfraHTTPAndSerialization(st, step, indent, sfx, arg, child); ok {
		return out
	}
	if out, ok := renderFlowStepInfraConcurrencyAndDelivery(st, step, indent, sfx, arg, child); ok {
		return out
	}
	return renderFlowStepInfraLegacy(st, step, indent, sfx, arg, child)
}
