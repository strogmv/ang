package mcp

import "testing"

func TestExtractPlanPatches(t *testing.T) {
	plan := map[string]any{
		"plan": map[string]any{
			"cue_apply_patch": []any{
				map[string]any{"path": "cue/domain/order.cue", "content": "Order: {}"},
			},
		},
	}
	patches := extractPlanPatches(plan)
	if len(patches) != 1 {
		t.Fatalf("len(patches)=%d, want 1", len(patches))
	}
	if patches[0]["path"] != "cue/domain/order.cue" {
		t.Fatalf("unexpected patch path: %#v", patches[0]["path"])
	}
}
