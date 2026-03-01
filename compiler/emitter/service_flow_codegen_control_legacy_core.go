package emitter

import (
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepControlLegacyCore(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string) (string, bool) {
	pad := strings.Repeat("\t", indent)

	if out, ok := renderFlowStepControlLegacyMappingLogic(st, step, pad, arg); ok {
		return out, true
	}
	if out, ok := renderFlowStepControlLegacyExecFS(st, step, pad, sfx, arg); ok {
		return out, true
	}
	return "", false
}
