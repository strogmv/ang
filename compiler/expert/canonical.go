package expert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// Canonicalize returns a deterministically ordered copy of report. Changes
// retain their original order because patch application may be order-sensitive.
func Canonicalize(report Report) (Report, error) {
	out := report
	out.KnowledgeVersions = sortedStrings(report.KnowledgeVersions)

	out.Findings = append([]Finding(nil), report.Findings...)
	for i := range out.Findings {
		out.Findings[i].FactIDs = sortedStrings(out.Findings[i].FactIDs)
		out.Findings[i].EvidenceIDs = sortedStrings(out.Findings[i].EvidenceIDs)
	}
	sort.SliceStable(out.Findings, func(i, j int) bool { return out.Findings[i].ID < out.Findings[j].ID })

	out.Proposals = append([]Proposal(nil), report.Proposals...)
	for i := range out.Proposals {
		proposal := &out.Proposals[i]
		proposal.RuleIDs = sortedStrings(proposal.RuleIDs)
		proposal.FindingIDs = sortedStrings(proposal.FindingIDs)
		proposal.Changes = append([]Change(nil), proposal.Changes...)
		for j := range proposal.Changes {
			value, err := canonicalRawMessage(proposal.Changes[j].Value)
			if err != nil {
				return Report{}, fmt.Errorf("proposals[%d].changes[%d].value: %w", i, j, err)
			}
			proposal.Changes[j].Value = value
		}
		preconditions, err := canonicalAssertions(proposal.Preconditions)
		if err != nil {
			return Report{}, fmt.Errorf("proposals[%d].preconditions: %w", i, err)
		}
		proposal.Preconditions = preconditions
		postconditions, err := canonicalAssertions(proposal.Postconditions)
		if err != nil {
			return Report{}, fmt.Errorf("proposals[%d].postconditions: %w", i, err)
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

// CanonicalJSON returns the stable JSON representation used by report storage
// and golden tests.
func CanonicalJSON(report Report) ([]byte, error) {
	canonical, err := Canonicalize(report)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

// Hash returns the lowercase SHA-256 of CanonicalJSON(report).
func Hash(report Report) (string, error) {
	data, err := CanonicalJSON(report)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalAssertions(assertions []Assertion) ([]Assertion, error) {
	out := append([]Assertion(nil), assertions...)
	for i := range out {
		expected, err := canonicalRawMessage(out[i].Expected)
		if err != nil {
			return nil, fmt.Errorf("assertions[%d].expected: %w", i, err)
		}
		out[i].Expected = expected
	}
	sort.SliceStable(out, func(i, j int) bool { return assertionKey(out[i]) < assertionKey(out[j]) })
	return out, nil
}

func canonicalRawMessage(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func assertionKey(assertion Assertion) string {
	return assertion.Kind + "\x00" + assertion.Subject + "\x00" + assertion.Predicate
}

func diagnosticKey(diagnostic Diagnostic) string {
	return diagnostic.Code + "\x00" + diagnostic.File + "\x00" + fmt.Sprintf("%09d", diagnostic.Line) + "\x00" + diagnostic.Message
}
