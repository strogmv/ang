package emitter

import (
	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepInfrastructure is the typed production entry for AI and
// project-planning actions. The mature renderers below already consume the
// predecoded Action through decodeCurrentActionAs; this adapter supplies only
// source metadata, never raw scalar arguments or legacy child steps.
func renderTypedStepInfrastructure(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	metadata := flowStepMetadata(step)
	emptyArg := func(string) string { return "" }
	emptyChild := func(string) []normalizer.FlowStep { return nil }

	if out, ok := renderFlowStepInfraClaude(st, metadata, indent, sfx, emptyArg, emptyChild); ok {
		return out, true
	}
	if out, ok := renderFlowStepMetaPlan(st, metadata, indent, sfx, emptyArg, emptyChild); ok {
		return out, true
	}
	if out, ok := renderFlowStepInfraOpenAI(st, metadata, indent, sfx, emptyArg, emptyChild); ok {
		return out, true
	}
	return renderFlowStepLocale(st, metadata, indent, sfx, emptyArg, emptyChild)
}
