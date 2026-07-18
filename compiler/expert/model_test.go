package expert

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestReportV1JSONContract(t *testing.T) {
	report := Report{
		Schema:            SchemaV1,
		Goal:              "project.audit",
		Status:            ReportAdvice,
		CompilerVersion:   "test",
		FactsHash:         "f00d",
		KnowledgeVersions: []string{},
		Findings: []Finding{{
			ID:         "finding.auth.required",
			Code:       "AUTH_REQUIRED",
			Severity:   "warning",
			Summary:    "endpoint has no actor requirement",
			Origin:     "compiler",
			Confidence: 1,
			Status:     FindingConfirmed,
		}},
		Proposals: []Proposal{{
			ID:   "proposal.auth.required",
			Goal: "project.audit",
			Changes: []Change{{
				Op: ChangeMerge, File: "cue/api/auth.cue", CUEPath: "services.Auth", Value: json.RawMessage(`{"auth": true}`), Rationale: "protect endpoint",
			}},
			Risk: RiskHigh, RequiresApproval: true,
		}},
		Trace: []RuleTrace{}, Verification: []VerificationResult{}, Diagnostics: []Diagnostic{},
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"schema":"ang/expert-report/v1","goal":"project.audit","status":"advice","compiler_version":"test","facts_hash":"f00d","knowledge_versions":[],"findings":[{"id":"finding.auth.required","code":"AUTH_REQUIRED","severity":"warning","summary":"endpoint has no actor requirement","origin":"compiler","confidence":1,"status":"confirmed"}],"proposals":[{"id":"proposal.auth.required","goal":"project.audit","changes":[{"op":"merge","file":"cue/api/auth.cue","cue_path":"services.Auth","value":{"auth":true},"rationale":"protect endpoint"}],"risk":"high","requires_approval":true}],"trace":[],"verification":[],"diagnostics":[]}`
	if string(data) != want {
		t.Fatalf("unexpected expert report v1 JSON\n got: %s\nwant: %s", data, want)
	}
}

func TestCanonicalJSONAndHashIgnoreReportOrdering(t *testing.T) {
	first := Report{
		Schema: SchemaV1, Goal: "project.audit", Status: ReportAdvice,
		KnowledgeVersions: []string{"security/v1", "core/v1"},
		Findings: []Finding{
			{ID: "finding.b", FactIDs: []string{"fact.2", "fact.1"}},
			{ID: "finding.a", FactIDs: []string{"fact.3"}},
		},
		Proposals: []Proposal{
			{ID: "proposal.b", RuleIDs: []string{"rule.b", "rule.a"}, Changes: []Change{{Value: json.RawMessage(`{"b":2,"a":1}`)}}},
			{ID: "proposal.a"},
		},
		Trace: []RuleTrace{{RuleID: "rule.b", MatchedFacts: []string{"fact.2", "fact.1"}}, {RuleID: "rule.a"}},
	}
	second := Report{
		Schema: SchemaV1, Goal: "project.audit", Status: ReportAdvice,
		KnowledgeVersions: []string{"core/v1", "security/v1"},
		Findings: []Finding{
			{ID: "finding.a", FactIDs: []string{"fact.3"}},
			{ID: "finding.b", FactIDs: []string{"fact.1", "fact.2"}},
		},
		Proposals: []Proposal{
			{ID: "proposal.a"},
			{ID: "proposal.b", RuleIDs: []string{"rule.a", "rule.b"}, Changes: []Change{{Value: json.RawMessage(` { "a" : 1, "b" : 2 } `)}}},
		},
		Trace: []RuleTrace{{RuleID: "rule.a"}, {RuleID: "rule.b", MatchedFacts: []string{"fact.1", "fact.2"}}},
	}
	firstJSON, err := CanonicalJSON(first)
	if err != nil {
		t.Fatalf("CanonicalJSON(first): %v", err)
	}
	secondJSON, err := CanonicalJSON(second)
	if err != nil {
		t.Fatalf("CanonicalJSON(second): %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("canonical JSON differs\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
	firstHash, err := Hash(first)
	if err != nil {
		t.Fatalf("Hash(first): %v", err)
	}
	secondHash, err := Hash(second)
	if err != nil {
		t.Fatalf("Hash(second): %v", err)
	}
	if firstHash != secondHash {
		t.Fatalf("hash mismatch: %s != %s", firstHash, secondHash)
	}
	if first.Findings[0].ID != "finding.b" || string(first.Proposals[0].Changes[0].Value) != `{"b":2,"a":1}` {
		t.Fatal("CanonicalJSON mutated its input")
	}
}

func TestValidateFactRejectsUnknownValueAndInvalidConfidence(t *testing.T) {
	err := ValidateFact(Fact{
		ID: "fact.auth", Kind: "auth", State: TruthUnknown, Confidence: 1.1, Value: json.RawMessage(`true`),
	})
	validation, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("ValidateFact() error = %T (%v), want *ValidationError", err, err)
	}
	if len(validation.Problems) != 2 || validation.Problems[0].Path != "confidence" || validation.Problems[1].Path != "value" {
		t.Fatalf("unexpected validation problems: %#v", validation.Problems)
	}
}

func TestReconcileReportStatusDoesNotDowngradeConflictOrFailure(t *testing.T) {
	conflict := []Finding{{Status: FindingConflict}}
	cases := []struct {
		name     string
		current  ReportStatus
		findings []Finding
		want     ReportStatus
	}{
		{name: "empty", current: ReportNoChange, want: ReportNoChange},
		{name: "advice", current: ReportNoChange, findings: []Finding{{Status: FindingConfirmed}}, want: ReportAdvice},
		{name: "conflict blocks", current: ReportAdvice, findings: conflict, want: ReportBlocked},
		{name: "failed wins", current: ReportFailed, findings: conflict, want: ReportFailed},
		{name: "existing block wins", current: ReportBlocked, findings: []Finding{{Status: FindingConfirmed}}, want: ReportBlocked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReconcileReportStatus(tc.current, tc.findings); got != tc.want {
				t.Fatalf("ReconcileReportStatus(%q, %#v) = %q, want %q", tc.current, tc.findings, got, tc.want)
			}
		})
	}
}

func TestValidateReportRejectsUnapprovedDelete(t *testing.T) {
	err := ValidateReport(Report{
		Schema: SchemaV1, Goal: "project.audit", Status: ReportAdvice,
		Proposals: []Proposal{{
			ID: "proposal.delete", Risk: RiskHigh, Changes: []Change{{Op: ChangeDelete, File: "cue/api/auth.cue", CUEPath: "services.Auth", Rationale: "remove obsolete intent"}},
		}},
	})
	validation, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("ValidateReport() error = %T (%v), want *ValidationError", err, err)
	}
	if len(validation.Problems) != 1 || validation.Problems[0].Path != "proposals[0].requires_approval" {
		t.Fatalf("unexpected validation problems: %#v", validation.Problems)
	}
}

func TestValidateReportRejectsProposalOutsideCUEIntent(t *testing.T) {
	err := ValidateReport(Report{
		Schema: SchemaV1, Goal: "project.audit", Status: ReportAdvice,
		Proposals: []Proposal{{
			ID: "proposal.outside", Risk: RiskLow, RequiresApproval: true,
			Changes: []Change{{Op: ChangeMerge, File: "/tmp/outside.cue", CUEPath: " ", Rationale: " ", Value: nil}},
		}},
	})
	validation, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("ValidateReport() error = %T (%v), want *ValidationError", err, err)
	}
	if len(validation.Problems) != 4 || validation.Problems[0].Path != "proposals[0].changes[0].cue_path" || validation.Problems[1].Path != "proposals[0].changes[0].file" || validation.Problems[2].Path != "proposals[0].changes[0].rationale" || validation.Problems[3].Path != "proposals[0].changes[0].value" {
		t.Fatalf("unexpected proposal validation problems: %#v", validation.Problems)
	}
}

func TestValidateReportRejectsDuplicateCanonicalIDs(t *testing.T) {
	err := ValidateReport(Report{
		Schema: SchemaV1, Goal: "project.audit", Status: ReportAdvice,
		KnowledgeVersions: []string{"security@v1", "security@v1"},
		Findings: []Finding{
			{ID: "finding.duplicate", Code: "DUPLICATE", Origin: "compiler", Confidence: 1, Status: FindingConfirmed},
			{ID: "finding.duplicate", Code: "DUPLICATE", Origin: "compiler", Confidence: 1, Status: FindingConfirmed},
		},
		Proposals: []Proposal{
			{ID: "proposal.duplicate", Risk: RiskLow},
			{ID: "proposal.duplicate", Risk: RiskLow},
		},
	})
	validation, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("ValidateReport() error = %T (%v), want *ValidationError", err, err)
	}
	if len(validation.Problems) != 3 || validation.Problems[0].Path != "findings[1].id" || validation.Problems[1].Path != "knowledge_versions[1]" || validation.Problems[2].Path != "proposals[1].id" {
		t.Fatalf("unexpected duplicate validation problems: %#v", validation.Problems)
	}
}

func TestProposalFromSuggestedFixesUsesExistingStructuredFixContract(t *testing.T) {
	diagnostic := normalizer.Warning{
		Code: "W_FLOW_HTTP_NO_TIMEOUT", Severity: "warning", Message: "HTTP request has no timeout",
		File: "cue/api/orders.cue", CUEPath: "Orders.Create.flow[2]",
		SuggestedFix: []normalizer.Fix{{
			Kind: "merge", Value: map[string]any{"timeout": "5s"}, Rationale: "bound external request duration",
		}},
	}
	proposal, err := ProposalFromSuggestedFixes("project.audit", diagnostic)
	if err != nil {
		t.Fatalf("ProposalFromSuggestedFixes: %v", err)
	}
	if proposal.ID == "" || proposal.Goal != "project.audit" || proposal.Risk != RiskMedium || !proposal.RequiresApproval || len(proposal.Changes) != 1 {
		t.Fatalf("unexpected proposal: %#v", proposal)
	}
	change := proposal.Changes[0]
	if change.Op != ChangeMerge || change.File != "cue/api/orders.cue" || change.CUEPath != "Orders.Create.flow[2]" || string(change.Value) != `{"timeout":"5s"}` || change.Rationale != "bound external request duration" {
		t.Fatalf("unexpected adapted change: %#v", change)
	}
	again, err := ProposalFromSuggestedFixes("project.audit", diagnostic)
	if err != nil || again.ID != proposal.ID {
		t.Fatalf("proposal ID is not deterministic: %#v, %v", again, err)
	}
}

func TestProposalFromSuggestedFixesRejectsUnsafeTarget(t *testing.T) {
	_, err := ProposalFromSuggestedFixes("project.audit", normalizer.Warning{
		Code: "UNSAFE", File: "/tmp/outside.cue", CUEPath: "outside", Message: "unsafe",
		SuggestedFix: []normalizer.Fix{{Op: "merge", Value: map[string]any{"x": true}}},
	})
	if err == nil || !strings.Contains(err.Error(), "relative .cue file within cue/") {
		t.Fatalf("error = %v, want rejected unsafe target", err)
	}
}

func TestAuditAdaptsCompilerDiagnosticsWithoutProposal(t *testing.T) {
	report := Audit(AuditInput{
		CompilerVersion: "test",
		Diagnostics: []normalizer.Warning{{
			Code: "FLOW_VARIABLE_UNKNOWN", Severity: "error", Message: "variable reply is not declared", File: "/tmp/project/cue/api/ops.cue", CUEPath: "services.Auth.Login.flow[0]",
		}},
	})
	if report.Schema != SchemaV1 || report.Goal != "project.audit" || report.Status != ReportBlocked {
		t.Fatalf("unexpected audit envelope: %#v", report)
	}
	if len(report.Findings) != 1 || report.Findings[0].Origin != "compiler" || report.Findings[0].RuleID != "" {
		t.Fatalf("unexpected audit finding: %#v", report.Findings)
	}
	if len(report.Proposals) != 0 {
		t.Fatalf("audit must not infer proposals: %#v", report)
	}
	if len(report.Trace) != 1 || report.Trace[0].Origin != "compiler" || report.Trace[0].ProducedIDs[0] != report.Findings[0].ID {
		t.Fatalf("audit trace does not explain compiler finding: %#v", report.Trace)
	}
	if err := ValidateReport(report); err != nil {
		t.Fatalf("ValidateReport(audit): %v", err)
	}
}

func TestAuditCanonicalizesCompilerDiagnosticOrder(t *testing.T) {
	first := []normalizer.Warning{
		{Code: "B", Severity: "warning", Message: "second"},
		{Code: "A", Severity: "warning", Message: "first"},
	}
	second := []normalizer.Warning{first[1], first[0]}
	firstJSON, err := CanonicalJSON(Audit(AuditInput{Diagnostics: first}))
	if err != nil {
		t.Fatalf("CanonicalJSON(first): %v", err)
	}
	secondJSON, err := CanonicalJSON(Audit(AuditInput{Diagnostics: second}))
	if err != nil {
		t.Fatalf("CanonicalJSON(second): %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("audit output is order-dependent\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

func TestAuditDeduplicatesCompilerDiagnostics(t *testing.T) {
	diagnostic := normalizer.Warning{Code: "FLOW_VARIABLE_UNKNOWN", Severity: "error", Message: "reply is missing"}
	report := Audit(AuditInput{Diagnostics: []normalizer.Warning{diagnostic, diagnostic}})
	if len(report.Findings) != 1 || len(report.Diagnostics) != 1 || len(report.Trace) != 1 {
		t.Fatalf("duplicate compiler diagnostics leaked into audit report: %#v", report)
	}
}

func TestValidateKnowledgePack(t *testing.T) {
	pack := KnowledgePack{
		Schema: KnowledgeSchemaV1, Name: "security", Version: "v1",
		Rules: []Rule{{
			ID: "security.auth.endpoint_requires_actor", Version: "v1", Priority: 100,
			Conditions:     []Condition{{Op: ConditionFactExists, FactKind: "endpoint"}},
			Conclusions:    []Conclusion{{Kind: "finding", Code: "AUTH_ACTOR_REQUIRED", Severity: "warning", Summary: "endpoint needs actor"}},
			BaseConfidence: 0.9, Risk: RiskHigh,
		}},
	}
	if err := ValidateKnowledgePack(pack); err != nil {
		t.Fatalf("ValidateKnowledgePack() error = %v", err)
	}
}

func TestValidateKnowledgePackRejectsUnsafeRule(t *testing.T) {
	err := ValidateKnowledgePack(KnowledgePack{
		Schema: KnowledgeSchemaV1, Name: "security", Version: "v1",
		Rules: []Rule{{
			ID: "security.unsafe", Version: "v1", BaseConfidence: 2, AutoApply: true, Risk: RiskLow,
			Conditions:  []Condition{{Op: "shell"}},
			Conclusions: []Conclusion{{Kind: "proposal", Severity: "fatal"}},
		}},
	})
	validation, ok := err.(*KnowledgeValidationError)
	if !ok {
		t.Fatalf("ValidateKnowledgePack() error = %T (%v), want *KnowledgeValidationError", err, err)
	}
	if len(validation.Problems) != 6 {
		t.Fatalf("problems = %#v, want six validation errors", validation.Problems)
	}
}

func TestValidateKnowledgePackRejectsIncompleteConditions(t *testing.T) {
	err := ValidateKnowledgePack(KnowledgePack{
		Schema: KnowledgeSchemaV1, Name: "security", Version: "v1",
		Rules: []Rule{{
			ID: "security.invalid_conditions", Version: "v1", BaseConfidence: 0.5, Risk: RiskLow,
			ConflictKeys: []string{"", "security.policy", "security.policy"},
			Conditions: []Condition{
				{Op: ConditionFactState},
				{Op: ConditionStringEqual, FactKind: "endpoint"},
				{Op: ConditionStringIn, FactKind: "endpoint", Values: []string{}},
			},
			Conclusions: []Conclusion{{Kind: "finding", Code: "INVALID", Severity: "warning", Summary: "invalid"}},
		}},
	})
	validation, ok := err.(*KnowledgeValidationError)
	if !ok {
		t.Fatalf("ValidateKnowledgePack() error = %T (%v), want *KnowledgeValidationError", err, err)
	}
	if len(validation.Problems) != 5 || validation.Problems[0].Path != "rules[0].conditions[0].state" || validation.Problems[1].Path != "rules[0].conditions[1].value" || validation.Problems[2].Path != "rules[0].conditions[2].values" || validation.Problems[3].Path != "rules[0].conflict_keys[0]" || validation.Problems[4].Path != "rules[0].conflict_keys[2]" {
		t.Fatalf("unexpected condition validation problems: %#v", validation.Problems)
	}
}

func TestValidateKnowledgePackRejectsIrrelevantConditionFields(t *testing.T) {
	err := ValidateKnowledgePack(KnowledgePack{
		Schema: KnowledgeSchemaV1, Name: "security", Version: "v1",
		Rules: []Rule{{
			ID: "security.invalid_fields", Version: "v1", BaseConfidence: 0.5, Risk: RiskLow,
			Conditions: []Condition{
				{Op: ConditionFactExists, State: TruthKnown},
				{Op: ConditionFactState, State: TruthKnown, Value: "unexpected"},
				{Op: ConditionStringEqual, Value: "enabled", Values: []string{"unexpected"}},
				{Op: ConditionStringIn, Value: "unexpected", Values: []string{"enabled"}},
			},
			Conclusions: []Conclusion{{Kind: "finding", Code: "INVALID", Severity: "warning", Summary: "invalid"}},
		}},
	})
	validation, ok := err.(*KnowledgeValidationError)
	if !ok {
		t.Fatalf("ValidateKnowledgePack() error = %T (%v), want *KnowledgeValidationError", err, err)
	}
	if len(validation.Problems) != 4 || validation.Problems[0].Path != "rules[0].conditions[0].state" || validation.Problems[1].Path != "rules[0].conditions[1].value" || validation.Problems[2].Path != "rules[0].conditions[2].values" || validation.Problems[3].Path != "rules[0].conditions[3].value" {
		t.Fatalf("unexpected irrelevant-field validation problems: %#v", validation.Problems)
	}
}
