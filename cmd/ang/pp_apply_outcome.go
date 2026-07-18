package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/expert"
	ppfacts "github.com/strogmv/ang/compiler/paymentprovider/facts"
)

func recordPPApplyOutcome(ctx context.Context, opts ppApplyOptions, validated ValidatedExpertReport, proposalID string) error {
	if strings.TrimSpace(opts.BaseURL) == "" {
		return fmt.Errorf("expert base URL is required")
	}
	runID, err := newOutcomeRunID()
	if err != nil {
		return err
	}
	envelopeAfter, err := ppfacts.Extract(ppfacts.ExtractOptions{
		ProjectPath: opts.ProjectPath,
		CueRoot:     opts.CueRoot,
		SchemaDir:   opts.SchemaDir,
	})
	if err != nil {
		return fmt.Errorf("extract facts after apply: %w", err)
	}
	factsAfterHash, err := ppfacts.Hash(envelopeAfter)
	if err != nil {
		return fmt.Errorf("hash facts after apply: %w", err)
	}
	factsBeforeHash := strings.TrimSpace(validated.ReportV2.FactsHash)
	if factsBeforeHash == "" {
		factsBeforeHash = strings.TrimSpace(validated.Report.FactsHash)
	}
	verification := []ExpertVerification{
		{Check: "pp_vet", Status: "passed"},
		{Check: "build", Status: "passed"},
		{Check: "post_facts", Status: "passed"},
	}
	blocking, unresolved := findingCodesFromValidated(validated)
	outcome := ExpertOutcomeRequest{
		Schema:                 "ang/expert-outcome/v1",
		RunID:                  runID,
		ScopeID:                envelopeAfter.ScopeID,
		Goal:                   "payment_provider.audit",
		CompilerVersion:        compiler.Version,
		FactsBeforeHash:        factsBeforeHash,
		ReportHash:             validated.ReportHash,
		KnowledgeVersions:      knowledgeVersionsFromValidated(validated),
		ProposalDecisions:      applyProposalDecisions(validated, proposalID),
		Verification:           verification,
		FactsAfterHash:         factsAfterHash,
		BlockingFindingCodes:   blocking,
		UnresolvedFindingCodes: unresolved,
		FinalStatus:            outcomeFinalStatusFromValidated("passed", "skipped", validated),
	}
	return RecordOutcome(ctx, ExpertClientConfig{
		BaseURL: opts.BaseURL,
		Timeout: 10 * time.Second,
	}, outcome)
}

func applyProposalDecisions(validated ValidatedExpertReport, acceptedProposalID string) []ExpertProposalDecision {
	switch validated.ReportSchema {
	case expert.SchemaV2:
		if len(validated.ReportV2.Proposals) == 0 {
			return nil
		}
		decisions := make([]ExpertProposalDecision, 0, len(validated.ReportV2.Proposals))
		for _, proposal := range validated.ReportV2.Proposals {
			decision := ExpertProposalDecision{
				ProposalID: proposal.ID,
				Decision:   "not_reviewed",
				ReasonCode: "apply_mode",
			}
			if proposal.ID == acceptedProposalID {
				decision.Decision = "accepted"
				decision.ReasonCode = "approved_apply"
			}
			decisions = append(decisions, decision)
		}
		return decisions
	default:
		return shadowProposalDecisions(&validated.Report)
	}
}
