package flowsem

import "testing"

func TestValidate_RepoGetForUpdateRequiresTx(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "repo.GetForUpdate"}})
	found := false
	for _, it := range issues {
		if it.Code == "TX_REQUIRED" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected TX_REQUIRED issue")
	}
}

func TestValidate_UpsertRequiresBranch(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "repo.Upsert",
		Args: map[string]any{
			"source": "User",
			"find":   "FindByEmail",
			"input":  "req.Email",
			"output": "user",
		},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_BRANCHES" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_BRANCHES issue")
	}
}

func TestValidate_UnknownAction(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "foo.Bar"}})
	found := false
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected UNKNOWN_ACTION issue")
	}
}

func TestValidate_FlowSwitchRequiresCases(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "flow.Switch",
		Args:   map[string]any{"value": "req.Role"},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_CASES" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_CASES issue")
	}
}

func TestValidate_ListEnrichSetFormat(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "list.Enrich",
		Args: map[string]any{
			"items":        "items",
			"lookupSource": "Company",
			"lookupInput":  "item.CompanyID",
			"set":          "AuthorName = Name",
		},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "INVALID_SET_FORMAT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected INVALID_SET_FORMAT issue")
	}
}

func TestValidate_FlowTryRequiresDo(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "flow.Try"}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_DO" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_DO issue")
	}
}

func TestValidate_FlowFallbackRequiresFallbackBranch(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action:   "flow.Fallback",
		Children: map[string][]Step{"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "nope"}}}},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_FALLBACK" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_FALLBACK issue")
	}
}

func TestValidate_FlowTimeoutRequiresDuration(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action:   "flow.Timeout",
		Children: map[string][]Step{"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "nope"}}}},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_DURATION" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_DURATION issue")
	}
}

func TestValidate_FlowSuggestNextRequiresOptions(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "flow.SuggestNext", Args: map[string]any{}}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_OPTIONS" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_OPTIONS issue")
	}
}

func TestValidate_PerformanceArgs(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{
			Action: "http.Call",
			Args:   map[string]any{"method": "GET", "url": "https://api.test", "attempts": 0},
		},
		{
			Action: "parallel.Run",
			Args:   map[string]any{"maxConcurrency": 0},
			Children: map[string][]Step{
				"_branches": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		},
		{
			Action: "queue.Enqueue",
			Args:   map[string]any{"subject": "events", "payload": "req", "timeoutMs": 0},
		},
	})

	want := map[string]bool{
		"INVALID_ATTEMPTS":        true,
		"INVALID_MAX_CONCURRENCY": true,
		"INVALID_TIMEOUT_MS":      true,
	}
	for _, it := range issues {
		delete(want, it.Code)
	}
	for code := range want {
		t.Fatalf("expected %s issue in %+v", code, issues)
	}
}
