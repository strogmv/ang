package ir

import (
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func TestConvertFlowStepsKeepsPositionsOfNestedSteps(t *testing.T) {
	steps := ConvertFlowSteps([]normalizer.FlowStep{{
		Action: "flow.If",
		File:   "cue/api/impl.cue", Line: 12, Column: 3, CUEPath: "Impls.A.flow[0]",
		Args: map[string]any{
			"_then": []normalizer.FlowStep{{Action: "logic.Call", File: "cue/api/impl.cue", Line: 14, Column: 5}},
		},
	}})
	if got := steps[0]; got.File != "cue/api/impl.cue" || got.Line != 12 || got.Column != 3 || got.CUEPath != "Impls.A.flow[0]" {
		t.Fatalf("step position = %+v", got)
	}
	if got := steps[0].Then[0]; got.Line != 14 || got.Column != 5 {
		t.Fatalf("nested step position = %+v", got)
	}
}
