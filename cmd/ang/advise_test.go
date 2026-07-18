package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/expert"
	"github.com/strogmv/ang/compiler/facts"
)

func TestRenderAdviceReportIsExplicitlyReadOnly(t *testing.T) {
	var out bytes.Buffer
	renderAdviceReport(&out, expert.Report{
		Goal: "project.audit", Status: expert.ReportAdvice,
		Findings: []expert.Finding{{Code: "FLOW_VARIABLE_UNKNOWN", Severity: "error", Summary: "reply is not declared"}},
	})
	text := out.String()
	for _, want := range []string{
		"ANG expert advice: project.audit",
		"Status: advice",
		"[ERROR] FLOW_VARIABLE_UNKNOWN: reply is not declared",
		"No changes proposed: project.audit is read-only.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("advice output missing %q:\n%s", want, text)
		}
	}
}

func TestRenderAdviceReportExplainsConflictBlock(t *testing.T) {
	var out bytes.Buffer
	renderAdviceReport(&out, expert.Report{
		Goal: "project.audit", Status: expert.ReportBlocked,
		Findings: []expert.Finding{{Code: "EXPERT_RULE_CONFLICT", Severity: "warning", Summary: "rules disagree", Status: expert.FindingConflict}},
	})
	if !strings.Contains(out.String(), "Decision blocked: resolve conflicting evidence or rules before acting.") {
		t.Fatalf("conflict guidance missing:\n%s", out.String())
	}
}

func TestBuildAdviceReportRejectsPackWithoutFacts(t *testing.T) {
	_, err := buildAdviceReport(".", "project.audit", "", []string{"testdata/security"})
	if err == nil || !strings.Contains(err.Error(), "--pack requires --facts") {
		t.Fatalf("error = %v, want --pack requires --facts", err)
	}
}

func TestRegisterKnowledgePackRejectsDuplicateVersion(t *testing.T) {
	seen := map[string]struct{}{}
	pack := expert.KnowledgePack{Name: "security", Version: "v1"}
	version, err := registerKnowledgePack(seen, pack)
	if err != nil || version != "security@v1" {
		t.Fatalf("registerKnowledgePack() = %q, %v", version, err)
	}
	if _, err := registerKnowledgePack(seen, pack); err == nil || !strings.Contains(err.Error(), `duplicate knowledge pack "security@v1"`) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestBuildAdviceReportRunsSecurityPackAgainstFacts(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	data, err := json.Marshal(facts.Envelope{
		Schema: facts.SchemaV1, SourceType: "fixture", SourcePath: "security.conf",
		SecurityRules: []facts.SecurityRule{{Scope: "global", Pattern: "public", Requirement: "permitAll"}},
	})
	if err != nil {
		t.Fatalf("marshal facts: %v", err)
	}
	factsPath := filepath.Join(t.TempDir(), "facts.json")
	if err := os.WriteFile(factsPath, data, 0o600); err != nil {
		t.Fatalf("write facts: %v", err)
	}
	report, err := buildAdviceReport(root, "project.audit", factsPath, []string{filepath.Join(root, "cue", "expert")})
	if err != nil {
		t.Fatalf("buildAdviceReport: %v", err)
	}
	if !containsAdviceFinding(report.Findings, "EXPERT_SECURITY_PERMIT_ALL") {
		t.Fatalf("security pack finding is missing: %#v", report.Findings)
	}
	if len(report.KnowledgeVersions) != 1 || report.KnowledgeVersions[0] != "security@v1" {
		t.Fatalf("unexpected knowledge versions: %#v", report.KnowledgeVersions)
	}
	if !containsKnowledgeTrace(report.Trace, "security.auth.permit_all_rule") {
		t.Fatalf("security pack trace is missing: %#v", report.Trace)
	}
}

func TestValidateExpertRuntimeResponseChecksProtocolAndFacts(t *testing.T) {
	request := expertRuntimeRequest{Schema: expertRequestSchema, RequestID: expertRequestID, Goal: "project.audit", CompilerVersion: "ang-test"}
	report := expert.Report{
		Schema: expert.SchemaV1, Goal: request.Goal, Status: expert.ReportNoChange,
		CompilerVersion: request.CompilerVersion, FactsHash: "facts-hash",
		KnowledgeVersions: []string{}, Findings: []expert.Finding{}, Proposals: []expert.Proposal{}, Trace: []expert.RuleTrace{}, Verification: []expert.VerificationResult{}, Diagnostics: []expert.Diagnostic{},
	}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	responseJSON, err := json.Marshal(expertRuntimeResponse{Schema: expertResponseSchema, RequestID: request.RequestID, RuntimeVersion: "0.1.0", Report: reportJSON})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateExpertRuntimeResponse(responseJSON, request, "facts-hash"); err != nil {
		t.Fatalf("validate response: %v", err)
	}
	if _, err := validateExpertRuntimeResponse(responseJSON, request, "different"); err == nil || !strings.Contains(err.Error(), "facts_hash") {
		t.Fatalf("facts hash mismatch error = %v", err)
	}
}

func TestExecuteExpertHTTPPostsVersionedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s", request.Method)
		}
		var input expertRuntimeRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.Schema != expertRequestSchema || input.RequestID != "http.test" {
			t.Fatalf("request = %+v", input)
		}
		report, _ := json.Marshal(expert.Report{Schema: expert.SchemaV1, Goal: input.Goal, Status: expert.ReportNoChange, CompilerVersion: input.CompilerVersion, FactsHash: "hash"})
		_ = json.NewEncoder(writer).Encode(expertRuntimeResponse{Schema: expertResponseSchema, RequestID: input.RequestID, RuntimeVersion: "test", Report: report})
	}))
	defer server.Close()
	data, err := executeExpertHTTP(server.URL, expertRuntimeRequest{Schema: expertRequestSchema, RequestID: "http.test", Goal: "project.audit", Facts: json.RawMessage(`{"schema":"ang/facts/v1"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty HTTP response")
	}
}

func containsAdviceFinding(findings []expert.Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func containsKnowledgeTrace(trace []expert.RuleTrace, ruleID string) bool {
	for _, entry := range trace {
		if entry.Origin == "knowledge" && entry.RuleID == ruleID && entry.Result == "matched" {
			return true
		}
	}
	return false
}
