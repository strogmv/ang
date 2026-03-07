package normalizer

import "testing"

func TestValidateFlowSteps_PolicyCheckRequiresCompanyID(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{
			Action: "policy.Check",
			Args: map[string]any{
				"policy": "CompanyAdminOnly",
				"user":   "currentUser",
			},
		},
	}
	policies := map[string]PolicyDef{
		"CompanyAdminOnly": {
			Name:        "CompanyAdminOnly",
			SameCompany: true,
			Roles:       []string{"owner", "admin"},
		},
	}

	issues := validateFlowSteps("AddCompanyCategory", "company", steps, nil, nil, policies, "strict", nil)
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_COMPANY_ID" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_COMPANY_ID diagnostic, got %+v", issues)
	}
}

func TestValidateFlowSteps_PolicyCheckUnknownPolicy(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{
			Action: "policy.Check",
			Args: map[string]any{
				"policy": "MissingPolicy",
				"user":   "currentUser",
			},
		},
	}
	policies := map[string]PolicyDef{
		"CompanyAdminOnly": {Name: "CompanyAdminOnly"},
	}

	issues := validateFlowSteps("AddCompanyCategory", "company", steps, nil, nil, policies, "strict", nil)
	found := false
	for _, it := range issues {
		if it.Code == "UNKNOWN_POLICY" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected UNKNOWN_POLICY diagnostic, got %+v", issues)
	}
}

func TestResolvePolicyChecks_InjectsPolicyMetadata(t *testing.T) {
	t.Parallel()

	n := New()
	n.Policies["CompanyAdminOnly"] = PolicyDef{
		Name:               "CompanyAdminOnly",
		Roles:              []string{"owner", "admin"},
		SameCompany:        true,
		AllowAdminOverride: true,
	}

	steps := []FlowStep{{
		Action: "policy.Check",
		Args: map[string]any{
			"policy": "CompanyAdminOnly",
			"user":   "currentUser",
		},
	}}
	out := n.resolvePolicyChecks(steps)
	if len(out) != 1 {
		t.Fatalf("unexpected steps len: %d", len(out))
	}
	if resolved, _ := out[0].Args["_policyResolved"].(bool); !resolved {
		t.Fatalf("expected _policyResolved=true, args=%+v", out[0].Args)
	}
	if gotPolicy, _ := out[0].Args["policy"].(string); gotPolicy != `"CompanyAdminOnly"` {
		t.Fatalf("expected quoted policy key, got %q", gotPolicy)
	}
}
