package normalizer

import "testing"

func TestValidateFlowSteps_ArchitectureModeSeverity(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Tender", Owner: "tender", BoundedContext: "tender"}}
	steps := []FlowStep{{Action: "repo.Find", Args: map[string]any{"source": "Tender", "input": "req.TenderID", "output": "tender"}}}

	strict := validateFlowSteps("CheckTender", "bids", steps, entities, nil, nil, "strict", nil)
	if len(strict) == 0 {
		t.Fatalf("expected diagnostics in strict mode")
	}
	if strict[0].Code != "ARCHITECTURE_VIOLATION" {
		t.Fatalf("expected ARCHITECTURE_VIOLATION, got %s", strict[0].Code)
	}
	if strict[0].Severity != "error" {
		t.Fatalf("expected error severity in strict mode, got %s", strict[0].Severity)
	}

	relaxed := validateFlowSteps("CheckTender", "bids", steps, entities, nil, nil, "relaxed", nil)
	if len(relaxed) == 0 {
		t.Fatalf("expected diagnostics in relaxed mode")
	}
	if relaxed[0].Severity != "warn" {
		t.Fatalf("expected warn severity in relaxed mode, got %s", relaxed[0].Severity)
	}
}

func TestValidateFlowSteps_ArchitectureAllowCrossService(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Tender", Owner: "tender", BoundedContext: "tender"}}
	steps := []FlowStep{{Action: "repo.Find", Args: map[string]any{"source": "Tender", "input": "req.TenderID", "output": "tender"}}}

	allow := map[string]map[string]struct{}{
		"bids": {"tender": {}},
	}
	warnings := validateFlowSteps("CheckTender", "bids", steps, entities, nil, nil, "strict", allow)
	for _, w := range warnings {
		if w.Code == "ARCHITECTURE_VIOLATION" {
			t.Fatalf("ARCHITECTURE_VIOLATION should be suppressed by allow_cross_service")
		}
	}
}

func TestValidateFlowSteps_NoViolationWithinSameBoundedContext(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{Name: "TenderCategory", Owner: "tender_category", BoundedContext: "tender"},
	}
	steps := []FlowStep{{Action: "repo.Find", Args: map[string]any{"source": "TenderCategory", "input": "req.ID", "output": "item"}}}

	warnings := validateFlowSteps("GetTenderCategory", "tender", steps, entities, nil, nil, "strict", nil)
	for _, w := range warnings {
		if w.Code == "ARCHITECTURE_VIOLATION" {
			t.Fatalf("unexpected ARCHITECTURE_VIOLATION in same bounded context: %+v", w)
		}
	}
}

func TestValidateFlowSteps_AggregateOwnsAllowsOwnedEntityAccess(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{
			Name:           "Tender",
			Owner:          "tender",
			BoundedContext: "tender",
			AggregateRoot:  true,
			Owns:           []string{"TenderCategory", "TenderInvite"},
		},
		{
			Name:           "TenderCategory",
			Owner:          "tender_category",
			BoundedContext: "catalog",
		},
	}
	steps := []FlowStep{{Action: "repo.Find", Args: map[string]any{"source": "TenderCategory", "input": "req.ID", "output": "item"}}}

	warnings := validateFlowSteps("GetTenderCategory", "tender", steps, entities, nil, nil, "strict", nil)
	for _, w := range warnings {
		if w.Code == "ARCHITECTURE_VIOLATION" {
			t.Fatalf("unexpected ARCHITECTURE_VIOLATION for owned aggregate entity: %+v", w)
		}
	}
}

func TestValidateFlowSteps_LegacySharedEntitiesNotImplicitlyAllowed(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{Name: "Company", Owner: "company", BoundedContext: "company"},
	}
	steps := []FlowStep{{Action: "repo.Find", Args: map[string]any{"source": "Company", "input": "req.CompanyID", "output": "company"}}}

	warnings := validateFlowSteps("GetCompany", "tender", steps, entities, nil, nil, "strict", nil)
	found := false
	for _, w := range warnings {
		if w.Code == "ARCHITECTURE_VIOLATION" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ARCHITECTURE_VIOLATION when accessing cross-context entity without read model")
	}
}

func TestValidateFlowSteps_SharedArchMetadataAllowsAccess(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{Name: "Company", Owner: "company", BoundedContext: "company", Metadata: map[string]any{"shared_arch": true}},
	}
	steps := []FlowStep{{Action: "repo.Find", Args: map[string]any{"source": "Company", "input": "req.CompanyID", "output": "company"}}}

	warnings := validateFlowSteps("GetCompany", "tender", steps, entities, nil, nil, "strict", nil)
	for _, w := range warnings {
		if w.Code == "ARCHITECTURE_VIOLATION" {
			t.Fatalf("ARCHITECTURE_VIOLATION should be suppressed for shared_arch entity")
		}
	}
}
