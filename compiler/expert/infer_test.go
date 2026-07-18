package expert

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestInferProducesFindingAndTraceForKnownFact(t *testing.T) {
	pack := testKnowledgePack(Rule{
		ID: "security.auth.endpoint_requires_actor", Version: "v1", Priority: 100, BaseConfidence: 0.9, Risk: RiskHigh,
		Conditions:  []Condition{{Op: ConditionStringEqual, FactKind: "endpoint", Predicate: "auth", Value: "missing"}},
		Conclusions: []Conclusion{{Kind: "finding", Code: "AUTH_ACTOR_REQUIRED", Severity: "warning", Summary: "endpoint needs actor"}},
	})
	result, err := Infer([]Fact{{
		ID: "fact.endpoint.login.auth", Kind: "endpoint", Subject: "Auth.Login", Predicate: "auth", State: TruthKnown, Confidence: 0.8, EvidenceIDs: []string{"evidence.login"}, Value: json.RawMessage(`"missing"`),
	}}, pack)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Code != "AUTH_ACTOR_REQUIRED" || result.Findings[0].Confidence != 0.8 {
		t.Fatalf("unexpected findings: %#v", result.Findings)
	}
	if len(result.Trace) != 1 || result.Trace[0].Result != "matched" || result.Trace[0].Origin != "knowledge" {
		t.Fatalf("unexpected trace: %#v", result.Trace)
	}
}

func TestInferTreatsMissingFactAsUnknown(t *testing.T) {
	pack := testKnowledgePack(Rule{
		ID: "security.auth.endpoint_requires_actor", Version: "v1", Priority: 100, BaseConfidence: 0.9, Risk: RiskHigh,
		Conditions:  []Condition{{Op: ConditionFactExists, FactKind: "endpoint"}},
		Conclusions: []Conclusion{{Kind: "finding", Code: "AUTH_ACTOR_REQUIRED", Severity: "warning", Summary: "endpoint needs actor"}},
	})
	result, err := Infer(nil, pack)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(result.Findings) != 0 || len(result.Trace) != 1 || result.Trace[0].Result != "unknown" {
		t.Fatalf("missing facts must not produce a finding: %#v", result)
	}
}

func TestInferReportsConflictFindingForConflictingFact(t *testing.T) {
	pack := testKnowledgePack(Rule{
		ID: "security.auth.endpoint_requires_actor", Version: "v1", Priority: 100, BaseConfidence: 0.9, Risk: RiskHigh,
		Conditions:  []Condition{{Op: ConditionFactExists, FactKind: "endpoint"}},
		Conclusions: []Conclusion{{Kind: "finding", Code: "AUTH_ACTOR_REQUIRED", Severity: "warning", Summary: "endpoint needs actor"}},
	})
	result, err := Infer([]Fact{{ID: "fact.endpoint", Kind: "endpoint", State: TruthConflict, Confidence: 0.5}}, pack)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Code != "EXPERT_FACT_CONFLICT" || result.Findings[0].Status != FindingConflict || result.Trace[0].Result != "conflict" {
		t.Fatalf("conflicting fact must produce a conflict finding: %#v", result)
	}
}

func TestInferMarksMutuallyExclusiveRuleResultsAsConflict(t *testing.T) {
	pack := testKnowledgePack(
		Rule{
			ID: "security.auth.public", Version: "v1", Priority: 100, BaseConfidence: 0.9, Risk: RiskHigh, ConflictKeys: []string{"security.auth.policy"},
			Conditions:  []Condition{{Op: ConditionStringEqual, FactKind: "policy", Predicate: "access", Value: "public"}},
			Conclusions: []Conclusion{{Kind: "finding", Code: "PUBLIC_ACCESS", Severity: "warning", Summary: "public access"}},
		},
		Rule{
			ID: "security.auth.private", Version: "v1", Priority: 100, BaseConfidence: 0.8, Risk: RiskHigh, ConflictKeys: []string{"security.auth.policy"},
			Conditions:  []Condition{{Op: ConditionStringEqual, FactKind: "policy", Predicate: "access", Value: "public"}},
			Conclusions: []Conclusion{{Kind: "finding", Code: "PRIVATE_ACCESS", Severity: "warning", Summary: "private access"}},
		},
	)
	result, err := Infer([]Fact{{
		ID: "fact.policy.access", Kind: "policy", Predicate: "access", State: TruthKnown, Confidence: 1, Value: json.RawMessage(`"public"`),
	}}, pack)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(result.Findings) != 3 {
		t.Fatalf("got %d findings, want two conflicted findings and one conflict explanation: %#v", len(result.Findings), result.Findings)
	}
	for _, finding := range result.Findings {
		if finding.Status != FindingConflict {
			t.Fatalf("finding %s has status %q, want conflict", finding.Code, finding.Status)
		}
	}
	if result.Trace[0].Result != "conflict" || result.Trace[1].Result != "conflict" || result.Trace[0].RejectedReason == "" || result.Trace[1].RejectedReason == "" {
		t.Fatalf("conflict must be explained in every participating trace: %#v", result.Trace)
	}
}

func TestInferCanEvaluateExplicitFactState(t *testing.T) {
	pack := testKnowledgePack(Rule{
		ID: "security.auth.endpoint_is_unknown", Version: "v1", Priority: 100, BaseConfidence: 0.9, Risk: RiskMedium,
		Conditions:  []Condition{{Op: ConditionFactState, FactKind: "endpoint", State: TruthUnknown}},
		Conclusions: []Conclusion{{Kind: "finding", Code: "AUTH_UNKNOWN", Severity: "info", Summary: "auth evidence is incomplete", Status: FindingUnknown}},
	})
	result, err := Infer([]Fact{{ID: "fact.endpoint", Kind: "endpoint", State: TruthUnknown, Confidence: 0.5}}, pack)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Status != FindingUnknown || result.Trace[0].Result != "matched" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestInferTreatsUnexpectedUnknownFactStateAsUnknown(t *testing.T) {
	pack := testKnowledgePack(Rule{
		ID: "security.auth.endpoint_is_known", Version: "v1", Priority: 100, BaseConfidence: 0.9, Risk: RiskMedium,
		Conditions:  []Condition{{Op: ConditionFactState, FactKind: "endpoint", State: TruthKnown}},
		Conclusions: []Conclusion{{Kind: "finding", Code: "AUTH_KNOWN", Severity: "info", Summary: "auth evidence is known"}},
	})
	result, err := Infer([]Fact{{ID: "fact.endpoint", Kind: "endpoint", State: TruthUnknown, Confidence: 0.5}}, pack)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(result.Findings) != 0 || result.Trace[0].Result != "unknown" {
		t.Fatalf("unknown state must remain unknown, got %#v", result)
	}
}

func TestInferFailsClosedForConflictingFactState(t *testing.T) {
	pack := testKnowledgePack(Rule{
		ID: "security.auth.endpoint_is_known", Version: "v1", Priority: 100, BaseConfidence: 0.9, Risk: RiskMedium,
		Conditions:  []Condition{{Op: ConditionFactState, FactKind: "endpoint", State: TruthKnown}},
		Conclusions: []Conclusion{{Kind: "finding", Code: "AUTH_KNOWN", Severity: "info", Summary: "auth evidence is known"}},
	})
	result, err := Infer([]Fact{
		{ID: "fact.endpoint.known", Kind: "endpoint", State: TruthKnown, Confidence: 1},
		{ID: "fact.endpoint.conflict", Kind: "endpoint", State: TruthConflict, Confidence: 0.5},
	}, pack)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Code != "EXPERT_FACT_CONFLICT" || result.Trace[0].Result != "conflict" {
		t.Fatalf("conflicting state must block the rule, got %#v", result)
	}
}

func TestInferFalseConditionWinsOverUnknownCondition(t *testing.T) {
	pack := testKnowledgePack(Rule{
		ID: "security.auth.requires_enabled_policy", Version: "v1", Priority: 100, BaseConfidence: 0.9, Risk: RiskMedium,
		Conditions: []Condition{
			{Op: ConditionFactExists, FactKind: "endpoint"},
			{Op: ConditionStringEqual, FactKind: "policy", Predicate: "status", Value: "enabled"},
		},
		Conclusions: []Conclusion{{Kind: "finding", Code: "POLICY_ENABLED", Severity: "warning", Summary: "policy is enabled"}},
	})
	result, err := Infer([]Fact{{
		ID: "fact.policy", Kind: "policy", Predicate: "status", State: TruthKnown, Confidence: 1, Value: json.RawMessage(`"disabled"`),
	}}, pack)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(result.Findings) != 0 || result.Trace[0].Result != "not_matched" {
		t.Fatalf("false condition must win over unknown condition: %#v", result)
	}
}

func TestSecurityKnowledgePackFindsExplicitPermitAllRule(t *testing.T) {
	pack, err := LoadKnowledgePack(filepath.Join(repoRoot(t), "cue", "expert"))
	if err != nil {
		t.Fatalf("LoadKnowledgePack: %v", err)
	}
	result, err := Infer([]Fact{{
		ID: "fact.security_rule.public", Kind: "security_rule", Subject: "global", Predicate: "public", State: TruthKnown, Confidence: 1, Value: json.RawMessage(`"permitAll"`),
	}}, pack)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Code != "EXPERT_SECURITY_PERMIT_ALL" || result.Findings[0].Confidence != 0.9 {
		t.Fatalf("unexpected security findings: %#v", result.Findings)
	}
}

func testKnowledgePack(rules ...Rule) KnowledgePack {
	return KnowledgePack{Schema: KnowledgeSchemaV1, Name: "security", Version: "v1", Rules: rules}
}
