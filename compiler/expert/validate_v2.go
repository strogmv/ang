package expert

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ValidateReportV2(report ReportV2) error {
	problems := make([]Problem, 0)
	if report.Schema != SchemaV2 {
		problems = append(problems, Problem{Path: "schema", Message: fmt.Sprintf("must equal %q", SchemaV2)})
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
			if err := validateIntentTarget(change.Target); err != nil {
				problems = append(problems, Problem{Path: changePath + ".target", Message: err.Error()})
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
			if err := validateOptionalHash(change.BeforeHash); err != nil {
				problems = append(problems, Problem{Path: changePath + ".before_hash", Message: err.Error()})
			}
		}
	}
	return validationError(problems)
}

func validateIntentTarget(target IntentTarget) error {
	kind := strings.TrimSpace(target.Kind)
	if kind != IntentTargetProjectCUERoot {
		return fmt.Errorf("kind must equal %q", IntentTargetProjectCUERoot)
	}
	path := strings.TrimSpace(target.RelativePath)
	if path == "" {
		return fmt.Errorf("relative_path must not be empty")
	}
	if filepath.IsAbs(path) || strings.Contains(path, `\`) {
		return fmt.Errorf("relative_path must be relative without backslashes")
	}
	clean := filepath.Clean(path)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return fmt.Errorf("relative_path must stay within project cue root")
	}
	if filepath.Ext(clean) != ".cue" {
		return fmt.Errorf("relative_path must be a .cue file")
	}
	return nil
}

func validateOptionalHash(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !hashPattern.MatchString(value) {
		return fmt.Errorf("must be lowercase sha256 hex")
	}
	return nil
}

// ValidateChangeScope ensures a v2 change resolves inside the project cue root.
func ValidateChangeScope(projectPath, cueRoot string, change ChangeV2) error {
	if err := validateIntentTarget(change.Target); err != nil {
		return err
	}
	root := filepath.Clean(filepath.Join(projectPath, cueRoot))
	target := filepath.Clean(filepath.Join(root, change.Target.RelativePath))
	rel, err := filepath.Rel(root, target)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." && change.Target.RelativePath != filepath.Base(change.Target.RelativePath) {
		return fmt.Errorf("target resolves outside project cue root")
	}
	if filepath.Ext(target) != ".cue" {
		return fmt.Errorf("target must address a .cue intent file")
	}
	return nil
}

// ValidateReportV2Scope validates every proposal change against a project cue root.
func ValidateReportV2Scope(projectPath, cueRoot string, report ReportV2) error {
	problems := make([]Problem, 0)
	for i, proposal := range report.Proposals {
		for j, change := range proposal.Changes {
			if err := ValidateChangeScope(projectPath, cueRoot, change); err != nil {
				problems = append(problems, Problem{
					Path:    fmt.Sprintf("proposals[%d].changes[%d].target", i, j),
					Message: err.Error(),
				})
			}
		}
	}
	return validationError(problems)
}

func ReportSchema(data json.RawMessage) (string, error) {
	var header struct {
		Schema string `json:"schema"`
	}
	if len(data) == 0 || !json.Valid(data) {
		return "", fmt.Errorf("report must be valid JSON")
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return "", fmt.Errorf("decode report schema: %w", err)
	}
	schema := strings.TrimSpace(header.Schema)
	if schema == "" {
		return "", fmt.Errorf("report.schema must not be empty")
	}
	return schema, nil
}
