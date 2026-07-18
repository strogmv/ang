package main

import (
	"testing"

	"github.com/strogmv/ang/compiler/expert"
)

func TestApplyProposalDecisionsMarksAccepted(t *testing.T) {
	validated := ValidatedExpertReport{
		ReportSchema: expert.SchemaV2,
		ReportV2: expert.ReportV2{
			Proposals: []expert.ProposalV2{
				{ID: "payment_provider.init_payout.declare"},
				{ID: "other.proposal"},
			},
		},
	}
	decisions := applyProposalDecisions(validated, "payment_provider.init_payout.declare")
	if len(decisions) != 2 {
		t.Fatalf("decisions = %d, want 2", len(decisions))
	}
	if decisions[0].Decision != "accepted" || decisions[0].ReasonCode != "approved_apply" {
		t.Fatalf("first decision = %+v", decisions[0])
	}
	if decisions[1].Decision != "not_reviewed" {
		t.Fatalf("second decision = %+v", decisions[1])
	}
}
