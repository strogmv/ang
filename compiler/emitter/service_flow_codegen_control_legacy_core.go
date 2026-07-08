package emitter

import (
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderFlowStepControlLegacyCore(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string) (string, bool) {
	pad := strings.Repeat("\t", indent)

	if out, ok := renderFlowStepControlLegacyExecFS(st, step, pad, sfx, arg); ok {
		return out, true
	}
	return "", false
}
