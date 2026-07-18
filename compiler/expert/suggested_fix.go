package expert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

// ProposalFromSuggestedFixes adapts the existing normalizer.Fix contract to
// one review-required expert Proposal. It performs no filesystem access: a
// later verifier must resolve the path and add BeforeHash before any apply.
func ProposalFromSuggestedFixes(goal string, warning normalizer.Warning) (Proposal, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		goal = "project.audit"
	}
	changes := make([]Change, 0, len(warning.SuggestedFix))
	for i, fix := range warning.SuggestedFix {
		change, err := changeFromSuggestedFix(warning, fix)
		if err != nil {
			return Proposal{}, fmt.Errorf("suggested_fix[%d]: %w", i, err)
		}
		changes = append(changes, change)
	}
	if len(changes) == 0 {
		return Proposal{}, fmt.Errorf("diagnostic %q has no structured suggested fixes", fallback(warning.Code, "ANG_DIAGNOSTIC"))
	}

	code := fallback(warning.Code, "ANG_DIAGNOSTIC")
	proposal := Proposal{
		Goal:             goal,
		RuleIDs:          []string{"compiler.diagnostic." + code},
		FindingIDs:       []string{compilerDiagnosticID(warning)},
		Changes:          changes,
		Risk:             riskFromDiagnosticSeverity(warning.Severity),
		RequiresApproval: true,
	}
	proposal.ID = suggestedFixProposalID(proposal, warning)
	if err := ValidateReport(Report{
		Schema: SchemaV1, Goal: goal, Status: ReportAdvice,
		Proposals: []Proposal{proposal},
	}); err != nil {
		return Proposal{}, err
	}
	return proposal, nil
}

func changeFromSuggestedFix(warning normalizer.Warning, fix normalizer.Fix) (Change, error) {
	op := strings.ToLower(strings.TrimSpace(fix.Op))
	if op == "" {
		op = strings.ToLower(strings.TrimSpace(fix.Kind))
	}
	if !validChangeOp(ChangeOp(op)) {
		return Change{}, fmt.Errorf("unsupported op %q", op)
	}
	if fix.Value == nil && op != string(ChangeDelete) {
		return Change{}, fmt.Errorf("%s requires a structured value", op)
	}
	var value json.RawMessage
	if fix.Value != nil {
		encoded, err := json.Marshal(fix.Value)
		if err != nil {
			return Change{}, fmt.Errorf("marshal value: %w", err)
		}
		value = encoded
	}
	return Change{
		Op:        ChangeOp(op),
		File:      firstNonEmpty(fix.File, warning.File),
		CUEPath:   firstNonEmpty(fix.CUEPath, warning.CUEPath),
		Value:     value,
		Rationale: firstNonEmpty(fix.Rationale, warning.Hint, warning.Message),
	}, nil
}

func suggestedFixProposalID(proposal Proposal, warning normalizer.Warning) string {
	changes, err := json.Marshal(proposal.Changes)
	if err != nil {
		panic("expert: marshal suggested-fix proposal changes: " + err.Error())
	}
	content := strings.Join([]string{
		"compiler_suggested_fix", proposal.Goal, compilerDiagnosticID(warning), string(changes),
	}, "\x00")
	sum := sha256.Sum256([]byte(content))
	return "proposal.compiler." + hex.EncodeToString(sum[:12])
}

func riskFromDiagnosticSeverity(severity string) RiskLevel {
	switch normalizedSeverity(severity) {
	case "error":
		return RiskHigh
	case "warning", "warn":
		return RiskMedium
	default:
		return RiskLow
	}
}
