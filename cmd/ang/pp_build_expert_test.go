package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/strogmv/ang/compiler/expert"
)

func TestParseOutputOptions_ExpertShadowRequiresBaseURL(t *testing.T) {
	_, err := parseOutputOptions([]string{"--expert-mode", "shadow"})
	if err == nil || !strings.Contains(err.Error(), "--expert-base-url") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseOutputOptions_ExpertAdviseRequiresBaseURL(t *testing.T) {
	_, err := parseOutputOptions([]string{"--expert-mode", "advise"})
	if err == nil || !strings.Contains(err.Error(), "--expert-base-url") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseOutputOptions_ExpertGateRequiresBaseURL(t *testing.T) {
	_, err := parseOutputOptions([]string{"--expert-mode", "gate"})
	if err == nil || !strings.Contains(err.Error(), "--expert-base-url") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseOutputOptions_ExpertGateAccepted(t *testing.T) {
	opts, err := parseOutputOptions([]string{"--expert-mode", "gate", "--expert-base-url", "http://127.0.0.1:8787"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if opts.ExpertMode != "gate" {
		t.Fatalf("mode = %q", opts.ExpertMode)
	}
}

func TestParseOutputOptions_ExpertFlagsRequireExpertMode(t *testing.T) {
	_, err := parseOutputOptions([]string{"--expert-base-url", "http://127.0.0.1:8787"})
	if err == nil || !strings.Contains(err.Error(), "expert flags require") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunPaymentProviderBuildOffModeDoesNotRequireExpert(t *testing.T) {
	dir := copyMinimalPPBuildFixture(t)
	result := runPaymentProviderBuild(dir, ".cue", filepath.Join(dir, ".ang", "templates"), "", OutputOptions{ExpertMode: "off"})
	if result.Err != nil {
		t.Fatalf("build failed: %v", result.Err)
	}
	if len(result.Manifest.Files) == 0 {
		t.Fatal("expected build manifest")
	}
}

func TestRunPaymentProviderBuildDryRunDoesNotMutateProject(t *testing.T) {
	dir := copyMinimalPPBuildFixture(t)
	marker := filepath.Join(dir, "KEEP")
	if err := os.WriteFile(marker, []byte("stay"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	dryRoot := t.TempDir()
	result := runPaymentProviderBuild(dir, ".cue", filepath.Join(dir, ".ang", "templates"), dryRoot, OutputOptions{
		ExpertMode: "off",
		DryRun:     true,
	})
	if result.Err != nil {
		t.Fatalf("dry-run build failed: %v", result.Err)
	}
	after, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("dry-run modified project marker")
	}
	if _, err := os.Stat(filepath.Join(dir, "testpp.go")); err == nil {
		t.Fatal("dry-run wrote generated file into project root")
	}
	if len(result.Manifest.Files) == 0 {
		t.Fatal("expected dry-run manifest")
	}
	if _, err := os.Stat(filepath.Join(dryRoot, "payment-provider", "testpp.go")); err != nil {
		t.Fatalf("expected generated output in dry-run dir: %v", err)
	}
}

func TestRunPaymentProviderBuildOffAndShadowSameManifest(t *testing.T) {
	offDir := copyMinimalPPBuildFixture(t)
	shadowDir := copyMinimalPPBuildFixture(t)

	offResult := runPaymentProviderBuild(offDir, ".cue", filepath.Join(offDir, ".ang", "templates"), "", OutputOptions{ExpertMode: "off"})
	if offResult.Err != nil {
		t.Fatalf("off build failed: %v", offResult.Err)
	}
	shadowResult := runPaymentProviderBuild(shadowDir, ".cue", filepath.Join(shadowDir, ".ang", "templates"), "", OutputOptions{
		ExpertMode:    "shadow",
		ExpertBaseURL: "http://127.0.0.1:1",
	})
	if shadowResult.Err != nil {
		t.Fatalf("shadow build failed: %v", shadowResult.Err)
	}
	offHash, err := offResult.Manifest.ManifestHash()
	if err != nil {
		t.Fatal(err)
	}
	shadowHash, err := shadowResult.Manifest.ManifestHash()
	if err != nil {
		t.Fatal(err)
	}
	if offHash != shadowHash {
		t.Fatalf("manifest mismatch: off=%s shadow=%s", offHash, shadowHash)
	}
}

func TestRunPaymentProviderBuildShadowContinuesWhenExpertUnavailable(t *testing.T) {
	dir := copyMinimalPPBuildFixture(t)
	result := runPaymentProviderBuild(dir, ".cue", filepath.Join(dir, ".ang", "templates"), "", OutputOptions{
		ExpertMode:    "shadow",
		ExpertBaseURL: "http://127.0.0.1:1",
	})
	if result.Err != nil {
		t.Fatalf("build should succeed despite expert failure: %v", result.Err)
	}
}

func TestRunPaymentProviderBuildShadowRecordsOutcome(t *testing.T) {
	dir := copyMinimalPPBuildFixture(t)
	var mu sync.Mutex
	var outcomes []ExpertOutcomeRequest

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case expertAnalyzePath:
			var input ExpertAnalyzeRequest
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(input.Facts)
			factsHash := hex.EncodeToString(sum[:])
			report, _ := json.Marshal(expert.Report{
				Schema: expert.SchemaV1, Goal: input.Goal, Status: expert.ReportAdvice,
				CompilerVersion: input.CompilerVersion, FactsHash: factsHash,
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
		case expertOutcomesPath:
			var input ExpertOutcomeRequest
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			outcomes = append(outcomes, input)
			mu.Unlock()
			_ = json.NewEncoder(writer).Encode(expertOutcomeResponse{
				Schema: "ang/expert-outcome-response/v1", RunID: input.RunID, Accepted: true,
			})
		default:
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
	}))
	defer server.Close()

	result := runPaymentProviderBuild(dir, ".cue", filepath.Join(dir, ".ang", "templates"), "", OutputOptions{
		ExpertMode:    "shadow",
		ExpertBaseURL: server.URL,
	})
	if result.Err != nil {
		t.Fatalf("build failed: %v", result.Err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(outcomes) != 1 {
		t.Fatalf("expected one outcome, got %d", len(outcomes))
	}
	if outcomes[0].FinalStatus != "advice" {
		t.Fatalf("final_status = %q", outcomes[0].FinalStatus)
	}
	if outcomes[0].Verification[0].Check != "build" || outcomes[0].Verification[0].Status != "passed" {
		t.Fatalf("verification = %+v", outcomes[0].Verification)
	}
	foundGoTest := false
	for _, check := range outcomes[0].Verification {
		if check.Check == "go_test" && check.Status == "skipped" {
			foundGoTest = true
		}
	}
	if !foundGoTest {
		t.Fatalf("expected go_test verification, got %+v", outcomes[0].Verification)
	}
	if outcomes[0].OutputManifestHash == "" {
		t.Fatal("expected output_manifest_hash")
	}
	if len(outcomes[0].UnresolvedFindingCodes) != 1 || outcomes[0].UnresolvedFindingCodes[0] != "PP_SCHEMA_DRIFT" {
		t.Fatalf("unresolved finding codes = %+v", outcomes[0].UnresolvedFindingCodes)
	}
}

func TestOutcomeFinalStatusRules(t *testing.T) {
	report := &expert.Report{Status: expert.ReportAdvice, Findings: []expert.Finding{{Code: "X"}}}
	if got := outcomeFinalStatus("passed", "skipped", report); got != "advice" {
		t.Fatalf("got %q", got)
	}
	if got := outcomeFinalStatus("failed", "skipped", report); got != "failed" {
		t.Fatalf("got %q", got)
	}
	if got := outcomeFinalStatus("passed", "failed", report); got != "failed" {
		t.Fatalf("got %q", got)
	}
	blocked := &expert.Report{Status: expert.ReportBlocked}
	if got := outcomeFinalStatus("passed", "skipped", blocked); got != "blocked" {
		t.Fatalf("got %q", got)
	}
	if got := outcomeFinalStatus("passed", "skipped", &expert.Report{}); got != "stable" {
		t.Fatalf("got %q", got)
	}
}

func copyMinimalPPBuildFixture(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "compiler", "paymentprovider", "testdata", "minimal")
	dir := t.TempDir()
	if err := copyTreePPBuild(src, dir); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	templatesSrc := filepath.Join("..", "..", "compiler", "paymentprovider", "testdata", "templates")
	templatesDst := filepath.Join(dir, ".ang", "templates")
	if err := copyTreePPBuild(templatesSrc, templatesDst); err != nil {
		t.Fatalf("copy templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ang.yaml"), []byte("templates_dir: .ang/templates\n"), 0o644); err != nil {
		t.Fatalf("write ang.yaml: %v", err)
	}
	return dir
}

func copyTreePPBuild(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
