package main

import (
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/compiler/expert"
)

func TestExpertGateBlocksBuildOnBlockedReport(t *testing.T) {
	blocked, reason := expertGateBlocksBuild(ValidatedExpertReport{
		ReportSchema: expert.SchemaV2,
		ReportV2: expert.ReportV2{
			Status: expert.ReportBlocked,
			Findings: []expert.Finding{{
				ID: "f1", Code: "X", Severity: "warning", Summary: "s", Origin: "knowledge", Confidence: 1, Status: expert.FindingConfirmed,
			}},
		},
	})
	if !blocked || reason == "" {
		t.Fatalf("blocked = %v reason = %q", blocked, reason)
	}
}

func TestExpertGateAllowsWarningFindings(t *testing.T) {
	blocked, _ := expertGateBlocksBuild(ValidatedExpertReport{
		ReportSchema: expert.SchemaV2,
		ReportV2: expert.ReportV2{
			Status: expert.ReportAdvice,
			Findings: []expert.Finding{{
				ID: "f1", Code: "PP_SCHEMA_DRIFT", Severity: "warning", Summary: "s", Origin: "knowledge", Confidence: 1, Status: expert.FindingConfirmed,
			}},
		},
	})
	if blocked {
		t.Fatal("expected warnings to pass gate")
	}
}

func TestExpertGateBlocksErrorFinding(t *testing.T) {
	blocked, reason := expertGateBlocksBuild(ValidatedExpertReport{
		ReportSchema: expert.SchemaV2,
		ReportV2: expert.ReportV2{
			Status: expert.ReportAdvice,
			Findings: []expert.Finding{{
				ID: "f1", Code: "PP_FATAL", Severity: "error", Summary: "s", Origin: "knowledge", Confidence: 1, Status: expert.FindingConfirmed,
			}},
		},
	})
	if !blocked || reason == "" {
		t.Fatalf("blocked = %v reason = %q", blocked, reason)
	}
}

func TestRunPaymentProviderBuildGateBlocksOnErrorFinding(t *testing.T) {
	dir := copyMinimalPPBuildFixture(t)
	// Gate with unavailable expert should fail-closed like advise.
	result := runPaymentProviderBuild(dir, ".cue", filepath.Join(dir, ".ang", "templates"), "", OutputOptions{
		ExpertMode:    "gate",
		ExpertBaseURL: "http://127.0.0.1:1",
	})
	if result.Err == nil {
		t.Fatal("expected gate build to fail when expert unavailable")
	}
}
