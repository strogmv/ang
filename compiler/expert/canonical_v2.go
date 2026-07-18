package expert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

func CanonicalizeV2(report ReportV2) (ReportV2, error) {
	out := report
	out.KnowledgeVersions = sortedStrings(report.KnowledgeVersions)

	out.Findings = append([]Finding(nil), report.Findings...)
	for i := range out.Findings {
		out.Findings[i].FactIDs = sortedStrings(out.Findings[i].FactIDs)
		out.Findings[i].EvidenceIDs = sortedStrings(out.Findings[i].EvidenceIDs)
	}
	sort.SliceStable(out.Findings, func(i, j int) bool { return out.Findings[i].ID < out.Findings[j].ID })

	out.Proposals = append([]ProposalV2(nil), report.Proposals...)
	for i := range out.Proposals {
		proposal := &out.Proposals[i]
		proposal.RuleIDs = sortedStrings(proposal.RuleIDs)
		proposal.FindingIDs = sortedStrings(proposal.FindingIDs)
		proposal.Changes = append([]ChangeV2(nil), proposal.Changes...)
		for j := range proposal.Changes {
			value, err := canonicalRawMessage(proposal.Changes[j].Value)
			if err != nil {
				return ReportV2{}, fmt.Errorf("proposals[%d].changes[%d].value: %w", i, j, err)
			}
			proposal.Changes[j].Value = value
		}
		preconditions, err := canonicalAssertions(proposal.Preconditions)
		if err != nil {
			return ReportV2{}, fmt.Errorf("proposals[%d].preconditions: %w", i, err)
		}
		proposal.Preconditions = preconditions
		postconditions, err := canonicalAssertions(proposal.Postconditions)
		if err != nil {
			return ReportV2{}, fmt.Errorf("proposals[%d].postconditions: %w", i, err)
		}
		proposal.Postconditions = postconditions
	}
	sort.SliceStable(out.Proposals, func(i, j int) bool { return out.Proposals[i].ID < out.Proposals[j].ID })

	out.Trace = append([]RuleTrace(nil), report.Trace...)
	for i := range out.Trace {
		out.Trace[i].MatchedFacts = sortedStrings(out.Trace[i].MatchedFacts)
		out.Trace[i].MissingFacts = sortedStrings(out.Trace[i].MissingFacts)
		out.Trace[i].ProducedIDs = sortedStrings(out.Trace[i].ProducedIDs)
	}
	sort.SliceStable(out.Trace, func(i, j int) bool { return out.Trace[i].RuleID < out.Trace[j].RuleID })

	out.Verification = append([]VerificationResult(nil), report.Verification...)
	sort.SliceStable(out.Verification, func(i, j int) bool {
		return out.Verification[i].Check+"\x00"+out.Verification[i].Status < out.Verification[j].Check+"\x00"+out.Verification[j].Status
	})
	out.Diagnostics = append([]Diagnostic(nil), report.Diagnostics...)
	sort.SliceStable(out.Diagnostics, func(i, j int) bool {
		return diagnosticKey(out.Diagnostics[i]) < diagnosticKey(out.Diagnostics[j])
	})
	return out, nil
}

func CanonicalJSONV2(report ReportV2) ([]byte, error) {
	canonical, err := CanonicalizeV2(report)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

func HashV2(report ReportV2) (string, error) {
	data, err := CanonicalJSONV2(report)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
