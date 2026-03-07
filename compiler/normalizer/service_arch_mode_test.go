package normalizer

import "testing"

func TestValidateFlowSteps_ArchitectureModeSeverity(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Tender", Owner: "tender"}}
	steps := []FlowStep{{Action: "repo.Find", Args: map[string]any{"source": "Tender", "input": "req.TenderID", "output": "tender"}}}

	strict := validateFlowSteps("CheckTender", "bids", steps, entities, nil, "strict", nil)
	if len(strict) == 0 {
		t.Fatalf("expected diagnostics in strict mode")
	}
	if strict[0].Code != "ARCHITECTURE_VIOLATION" {
		t.Fatalf("expected ARCHITECTURE_VIOLATION, got %s", strict[0].Code)
	}
	if strict[0].Severity != "error" {
		t.Fatalf("expected error severity in strict mode, got %s", strict[0].Severity)
	}

	relaxed := validateFlowSteps("CheckTender", "bids", steps, entities, nil, "relaxed", nil)
	if len(relaxed) == 0 {
		t.Fatalf("expected diagnostics in relaxed mode")
	}
	if relaxed[0].Severity != "warn" {
		t.Fatalf("expected warn severity in relaxed mode, got %s", relaxed[0].Severity)
	}
}

func TestValidateFlowSteps_ArchitectureAllowCrossService(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Tender", Owner: "tender"}}
	steps := []FlowStep{{Action: "repo.Find", Args: map[string]any{"source": "Tender", "input": "req.TenderID", "output": "tender"}}}

	allow := map[string]map[string]struct{}{
		"bids": {"tender": {}},
	}
	warnings := validateFlowSteps("CheckTender", "bids", steps, entities, nil, "strict", allow)
	for _, w := range warnings {
		if w.Code == "ARCHITECTURE_VIOLATION" {
			t.Fatalf("ARCHITECTURE_VIOLATION should be suppressed by allow_cross_service")
		}
	}
}
