package mcp

import "testing"

func TestBuildGoalPlan_MarketplaceBlueprint(t *testing.T) {
	plan, err := buildGoalPlan("marketplace with orders, payments, notifications")
	if err != nil {
		t.Fatalf("buildGoalPlan error: %v", err)
	}
	root, ok := plan["plan"].(map[string]any)
	if !ok {
		t.Fatalf("missing plan object: %#v", plan)
	}

	entities, ok := root["entities"].([]map[string]any)
	if !ok || len(entities) == 0 {
		t.Fatalf("expected entities in marketplace plan")
	}
	hasOrder := false
	for _, e := range entities {
		if n, _ := e["name"].(string); n == "Order" {
			hasOrder = true
			if _, ok := e["fsm"].(map[string]any); !ok {
				t.Fatalf("Order entity missing fsm in blueprint")
			}
		}
	}
	if !hasOrder {
		t.Fatalf("expected Order entity in marketplace blueprint")
	}

	patches, ok := root["cue_apply_patch"].([]map[string]any)
	if !ok || len(patches) < 4 {
		t.Fatalf("expected cue_apply_patch templates, got %#v", root["cue_apply_patch"])
	}
	patterns, ok := root["pattern_sources"].([]string)
	if !ok || len(patterns) == 0 {
		t.Fatalf("expected non-empty pattern_sources")
	}
	if est, ok := root["estimated_iterations"].(int); !ok || est != 2 {
		t.Fatalf("expected estimated_iterations=2, got %#v", root["estimated_iterations"])
	}
}
