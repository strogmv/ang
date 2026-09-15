package emitter

import (
	"testing"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

func TestIRFlowStepsToNormalizerKeepsPositions(t *testing.T) {
	steps := irFlowStepsToNormalizer([]ir.FlowStep{{
		Action: "flow.If",
		File:   "cue/api/impl.cue", Line: 12, Column: 3, CUEPath: "Impls.A.flow[0]",
		Then: []ir.FlowStep{{Action: "logic.Call", File: "cue/api/impl.cue", Line: 14}},
	}})
	if got := steps[0]; got.File != "cue/api/impl.cue" || got.Line != 12 || got.Column != 3 || got.CUEPath != "Impls.A.flow[0]" {
		t.Fatalf("step position = %+v", got)
	}
	then := steps[0].Args["_then"].([]normalizer.FlowStep)
	if then[0].Line != 14 {
		t.Fatalf("nested step position = %+v", then[0])
	}
}
