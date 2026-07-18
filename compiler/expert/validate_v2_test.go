package expert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateReportV2RejectsOutsideProjectCUETarget(t *testing.T) {
	err := ValidateReportV2(ReportV2{
		Schema: SchemaV2, Goal: "payment_provider.audit", Status: ReportAdvice,
		Proposals: []ProposalV2{{
			ID: "proposal.outside", Risk: RiskLow, RequiresApproval: true,
			Changes: []ChangeV2{{
				Op: ChangeMerge,
				Target: IntentTarget{
					Kind:         IntentTargetProjectCUERoot,
					RelativePath: "../outside.cue",
				},
				CUEPath:   "operations.init_payout",
				Value:     json.RawMessage(`{"enabled":true}`),
				Rationale: "test",
			}},
		}},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateReportV2ScopeRejectsGeneratedGoPath(t *testing.T) {
	report := ReportV2{
		Schema: SchemaV2, Goal: "payment_provider.audit", Status: ReportAdvice,
		Proposals: []ProposalV2{{
			ID: "proposal.go", Risk: RiskLow, RequiresApproval: true,
			Changes: []ChangeV2{{
				Op: ChangeMerge,
				Target: IntentTarget{
					Kind:         IntentTargetProjectCUERoot,
					RelativePath: "provider.go",
				},
				CUEPath:   "package",
				Value:     json.RawMessage(`{}`),
				Rationale: "test",
			}},
		}},
	}
	if err := ValidateReportV2(report); err == nil {
		t.Fatal("expected schema validation error for non-cue target")
	}
}

func TestValidateReportV2ScopeAllowsProviderCueWithinRoot(t *testing.T) {
	root := t.TempDir()
	cueRoot := filepath.Join(root, ".cue")
	if err := os.MkdirAll(cueRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cueRoot, "provider.cue"), []byte("package provider\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := ReportV2{
		Schema: SchemaV2, Goal: "payment_provider.audit", Status: ReportAdvice,
		Proposals: []ProposalV2{{
			ID: "proposal.ok", Risk: RiskMedium, RequiresApproval: true,
			Changes: []ChangeV2{{
				Op: ChangeMerge,
				Target: IntentTarget{
					Kind:         IntentTargetProjectCUERoot,
					RelativePath: "provider.cue",
				},
				CUEPath:   "operations.init_payout",
				Value:     json.RawMessage(`{"enabled":true}`),
				Rationale: "declare payout operation",
			}},
		}},
	}
	if err := ValidateReportV2(report); err != nil {
		t.Fatalf("ValidateReportV2: %v", err)
	}
	if err := ValidateReportV2Scope(root, ".cue", report); err != nil {
		t.Fatalf("ValidateReportV2Scope: %v", err)
	}
}
