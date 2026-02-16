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
