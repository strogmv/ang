package emitter

import (
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepHTTPCall is the typed production writer for http.Call.
func renderTypedStepHTTPCall(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	typed, err := typedActionAs[flowir.HTTPCall](step)
	if err != nil {
		return renderInvalidFlowStepConfig(st, strings.Repeat("\t", indent), step.Name, err.Error()), true
	}
	if out, ok := renderFlowHTTPCallAST(st, typed, indent, sfx); ok {
		return out, true
	}
	return renderInvalidFlowStepConfig(st, strings.Repeat("\t", indent), step.Name, "http.Call contains an invalid Go expression"), true
}
