package expert

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

// ValidationError reports stable, path-addressable contract violations.
type ValidationError struct {
	Problems []Problem
}

type Problem struct {
	Path    string
	Message string
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, problem := range e.Problems {
		parts = append(parts, problem.Path+": "+problem.Message)
	}
	return "invalid " + SchemaV1 + " document: " + strings.Join(parts, "; ")
}

func ValidateFact(fact Fact) error {
	problems := make([]Problem, 0)
	if strings.TrimSpace(fact.ID) == "" {
		problems = append(problems, Problem{Path: "id", Message: "must not be empty"})
	}
	if strings.TrimSpace(fact.Kind) == "" {
		problems = append(problems, Problem{Path: "kind", Message: "must not be empty"})
	}
	if !validTruthState(fact.State) {
		problems = append(problems, Problem{Path: "state", Message: "must be known, unknown, or conflict"})
	}
	if err := validateConfidence(fact.Confidence); err != nil {
		problems = append(problems, Problem{Path: "confidence", Message: err.Error()})
	}
	if fact.State == TruthUnknown && len(fact.Value) != 0 {
		problems = append(problems, Problem{Path: "value", Message: "must be empty when state is unknown"})
	}
	if err := validateRawJSON(fact.Value); err != nil {
		problems = append(problems, Problem{Path: "value", Message: err.Error()})
	}
	return validationError(problems)
}

func ValidateReport(report Report) error {
	problems := make([]Problem, 0)
	if report.Schema != SchemaV1 {
		problems = append(problems, Problem{Path: "schema", Message: fmt.Sprintf("must equal %q", SchemaV1)})
	}
	if strings.TrimSpace(report.Goal) == "" {
		problems = append(problems, Problem{Path: "goal", Message: "must not be empty"})
	}
	if !validReportStatus(report.Status) {
		problems = append(problems, Problem{Path: "status", Message: "must be no_change, advice, blocked, verified, or failed"})
	}
	seenKnowledgeVersions := map[string]struct{}{}
	for i, version := range report.KnowledgeVersions {
		path := fmt.Sprintf("knowledge_versions[%d]", i)
		if strings.TrimSpace(version) == "" {
			problems = append(problems, Problem{Path: path, Message: "must not be empty"})
			continue
		}
		if _, exists := seenKnowledgeVersions[version]; exists {
			problems = append(problems, Problem{Path: path, Message: "must be unique"})
			continue
		}
		seenKnowledgeVersions[version] = struct{}{}
	}
	seenFindingIDs := map[string]struct{}{}
	for i, finding := range report.Findings {
		path := fmt.Sprintf("findings[%d]", i)
		if strings.TrimSpace(finding.ID) == "" {
			problems = append(problems, Problem{Path: path + ".id", Message: "must not be empty"})
		} else if _, exists := seenFindingIDs[finding.ID]; exists {
			problems = append(problems, Problem{Path: path + ".id", Message: "must be unique"})
		} else {
			seenFindingIDs[finding.ID] = struct{}{}
		}
		if strings.TrimSpace(finding.Code) == "" {
			problems = append(problems, Problem{Path: path + ".code", Message: "must not be empty"})
		}
		if !validFindingStatus(finding.Status) {
			problems = append(problems, Problem{Path: path + ".status", Message: "must be confirmed, hypothesis, unknown, or conflict"})
		}
		if err := validateConfidence(finding.Confidence); err != nil {
			problems = append(problems, Problem{Path: path + ".confidence", Message: err.Error()})
		}
		if strings.TrimSpace(finding.RuleID) == "" && finding.Origin != "compiler" {
			problems = append(problems, Problem{Path: path + ".rule_id", Message: "must not be empty unless origin is compiler"})
		}
	}
	seenProposalIDs := map[string]struct{}{}
	for i, proposal := range report.Proposals {
		path := fmt.Sprintf("proposals[%d]", i)
		if strings.TrimSpace(proposal.ID) == "" {
			problems = append(problems, Problem{Path: path + ".id", Message: "must not be empty"})
		} else if _, exists := seenProposalIDs[proposal.ID]; exists {
			problems = append(problems, Problem{Path: path + ".id", Message: "must be unique"})
		} else {
			seenProposalIDs[proposal.ID] = struct{}{}
		}
		if !validRiskLevel(proposal.Risk) {
			problems = append(problems, Problem{Path: path + ".risk", Message: "must be low, medium, high, or critical"})
		}
		for j, change := range proposal.Changes {
			changePath := fmt.Sprintf("%s.changes[%d]", path, j)
			if !validChangeOp(change.Op) {
				problems = append(problems, Problem{Path: changePath + ".op", Message: "must be insert, merge, replace, or delete"})
			}
			if !validCUEIntentPath(change.File) {
				problems = append(problems, Problem{Path: changePath + ".file", Message: "must be a relative .cue file within cue/"})
			}
			if strings.TrimSpace(change.CUEPath) == "" {
				problems = append(problems, Problem{Path: changePath + ".cue_path", Message: "must not be empty"})
			}
			if strings.TrimSpace(change.Rationale) == "" {
				problems = append(problems, Problem{Path: changePath + ".rationale", Message: "must not be empty"})
			}
			if change.Op == ChangeDelete && !proposal.RequiresApproval {
				problems = append(problems, Problem{Path: path + ".requires_approval", Message: "must be true for a delete change"})
			}
			if change.Op != ChangeDelete && len(change.Value) == 0 {
				problems = append(problems, Problem{Path: changePath + ".value", Message: "must be present for insert, merge, or replace"})
			}
			if err := validateRawJSON(change.Value); err != nil {
				problems = append(problems, Problem{Path: changePath + ".value", Message: err.Error()})
			}
		}
	}
	return validationError(problems)
}

func validCUEIntentPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, `\`) {
		return false
	}
	clean := filepath.Clean(path)
	return strings.HasPrefix(clean, "cue/") && filepath.Ext(clean) == ".cue"
}

func validationError(problems []Problem) error {
	if len(problems) == 0 {
		return nil
	}
	sort.Slice(problems, func(i, j int) bool {
		if problems[i].Path == problems[j].Path {
			return problems[i].Message < problems[j].Message
		}
		return problems[i].Path < problems[j].Path
	})
	return &ValidationError{Problems: problems}
}

func validateConfidence(confidence float64) error {
	if math.IsNaN(confidence) || math.IsInf(confidence, 0) || confidence < 0 || confidence > 1 {
		return fmt.Errorf("must be in [0, 1]")
	}
	return nil
}

func validateRawJSON(raw json.RawMessage) error {
	if len(raw) == 0 || json.Valid(raw) {
		return nil
	}
	return fmt.Errorf("must be valid JSON")
}

func validTruthState(state TruthState) bool {
	return state == TruthKnown || state == TruthUnknown || state == TruthConflict
}

func validFindingStatus(status FindingStatus) bool {
	return status == FindingConfirmed || status == FindingHypothesis || status == FindingUnknown || status == FindingConflict
}

func validReportStatus(status ReportStatus) bool {
	return status == ReportNoChange || status == ReportAdvice || status == ReportBlocked || status == ReportVerified || status == ReportFailed
}

func validRiskLevel(risk RiskLevel) bool {
	return risk == RiskLow || risk == RiskMedium || risk == RiskHigh || risk == RiskCritical
}

func validChangeOp(op ChangeOp) bool {
	return op == ChangeInsert || op == ChangeMerge || op == ChangeReplace || op == ChangeDelete
}
