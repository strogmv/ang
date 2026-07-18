package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/expert"
)

func TestValidateExpertBaseURLAllowsLoopbackHTTP(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:8787", "http://localhost:8787", "http://[::1]:8787"} {
		if _, err := validateExpertBaseURL(raw); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
}

func TestValidateExpertBaseURLRejectsNonLoopbackHTTP(t *testing.T) {
	if _, err := validateExpertBaseURL("http://example.com:8787"); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateExpertModeAllowsGate(t *testing.T) {
	for _, mode := range []string{"advise", "gate"} {
		if err := validateExpertMode(mode); err != nil {
			t.Fatalf("mode %q: %v", mode, err)
		}
	}
	if err := validateExpertMode("unknown"); err == nil || !strings.Contains(err.Error(), "unsupported expert mode") {
		t.Fatalf("error = %v", err)
	}
}

func TestAnalyzeUsesBaseURLAndValidatesReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != expertAnalyzePath {
			t.Fatalf("path = %s", request.URL.Path)
		}
		var input ExpertAnalyzeRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		report, _ := json.Marshal(expert.Report{
			Schema: expert.SchemaV1, Goal: input.Goal, Status: expert.ReportAdvice,
			CompilerVersion: input.CompilerVersion, FactsHash: "abc123",
			KnowledgeVersions: []string{"payment-provider.core@0.1.0"},
			Findings: []expert.Finding{{
				ID: "finding.1", Code: "PP_SCHEMA_DRIFT", Severity: "warning",
				Summary: "bundled payment-provider schema differs from project copy",
				Origin: "knowledge", RuleID: "payment_provider.schema_drift", Confidence: 1, Status: expert.FindingConfirmed,
			}},
		})
		_ = json.NewEncoder(writer).Encode(expertRuntimeResponse{
			Schema: expertResponseSchema, RequestID: input.RequestID, RuntimeVersion: "test", Report: report,
		})
	}))
	defer server.Close()

	validated, err := Analyze(context.Background(), ExpertClientConfig{BaseURL: server.URL}, ExpertAnalyzeRequest{
		Schema: expertRequestSchema, RequestID: "pp.expert.test", Goal: "payment_provider.audit",
		CompilerVersion: "ang-test", Facts: json.RawMessage(`{"schema":"ang/payment-provider-facts/v1"}`),
	}, "abc123")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if validated.RuntimeVersion != "test" || len(validated.Report.Findings) != 1 {
		t.Fatalf("validated = %+v", validated)
	}
}

func TestAnalyzeRejectsFactsHashMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input ExpertAnalyzeRequest
		_ = json.NewDecoder(request.Body).Decode(&input)
		report, _ := json.Marshal(expert.Report{
			Schema: expert.SchemaV1, Goal: input.Goal, Status: expert.ReportNoChange,
			CompilerVersion: input.CompilerVersion, FactsHash: "wrong",
		})
		_ = json.NewEncoder(writer).Encode(expertRuntimeResponse{
			Schema: expertResponseSchema, RequestID: input.RequestID, RuntimeVersion: "test", Report: report,
		})
	}))
	defer server.Close()

	_, err := Analyze(context.Background(), ExpertClientConfig{BaseURL: server.URL}, ExpertAnalyzeRequest{
		Schema: expertRequestSchema, RequestID: "pp.expert.test", Goal: "payment_provider.audit",
		CompilerVersion: "ang-test", Facts: json.RawMessage(`{"schema":"ang/payment-provider-facts/v1"}`),
	}, "expected")
	if err == nil || !strings.Contains(err.Error(), "facts_hash") {
		t.Fatalf("error = %v", err)
	}
}
