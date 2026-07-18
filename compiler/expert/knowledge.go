package expert

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const KnowledgeSchemaV1 = "ang/knowledge-pack/v1"

type ConditionOp string

const (
	ConditionFactExists  ConditionOp = "fact_exists"
	ConditionFactState   ConditionOp = "fact_state"
	ConditionStringEqual ConditionOp = "string_equals"
	ConditionStringIn    ConditionOp = "string_in"
)

// Condition is a deliberately bounded predicate over normalized Facts.
type Condition struct {
	Op        ConditionOp `json:"op"`
	FactKind  string      `json:"fact_kind,omitempty"`
	Subject   string      `json:"subject,omitempty"`
	Predicate string      `json:"predicate,omitempty"`
	State     TruthState  `json:"state,omitempty"`
	Value     string      `json:"value,omitempty"`
	Values    []string    `json:"values,omitempty"`
}

type Conclusion struct {
	Kind     string        `json:"kind"`
	Code     string        `json:"code"`
	Severity string        `json:"severity"`
	Summary  string        `json:"summary"`
	Status   FindingStatus `json:"status,omitempty"`
	Risk     RiskLevel     `json:"risk,omitempty"`
}

type Rule struct {
	ID             string       `json:"id"`
	Version        string       `json:"version"`
	Description    string       `json:"description,omitempty"`
	Priority       int          `json:"priority"`
	RequiredKinds  []string     `json:"required_kinds,omitempty"`
	Conditions     []Condition  `json:"conditions"`
	Conclusions    []Conclusion `json:"conclusions"`
	ConflictKeys   []string     `json:"conflict_keys,omitempty"`
	BaseConfidence float64      `json:"base_confidence"`
	AutoApply      bool         `json:"auto_apply"`
	Risk           RiskLevel    `json:"risk"`
}

type KnowledgePack struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Rules       []Rule `json:"rules"`
}

type KnowledgeValidationError struct {
	Problems []Problem
}

func (e *KnowledgeValidationError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, problem := range e.Problems {
		parts = append(parts, problem.Path+": "+problem.Message)
	}
	return "invalid " + KnowledgeSchemaV1 + " document: " + strings.Join(parts, "; ")
}

func ValidateKnowledgePack(pack KnowledgePack) error {
	problems := make([]Problem, 0)
	if pack.Schema != KnowledgeSchemaV1 {
		problems = append(problems, Problem{Path: "schema", Message: fmt.Sprintf("must equal %q", KnowledgeSchemaV1)})
	}
	if strings.TrimSpace(pack.Name) == "" {
		problems = append(problems, Problem{Path: "name", Message: "must not be empty"})
	}
	if strings.TrimSpace(pack.Version) == "" {
		problems = append(problems, Problem{Path: "version", Message: "must not be empty"})
	}
	seenRuleIDs := map[string]struct{}{}
	for i, rule := range pack.Rules {
		path := fmt.Sprintf("rules[%d]", i)
		if strings.TrimSpace(rule.ID) == "" {
			problems = append(problems, Problem{Path: path + ".id", Message: "must not be empty"})
		} else if _, exists := seenRuleIDs[rule.ID]; exists {
			problems = append(problems, Problem{Path: path + ".id", Message: "must be unique within a knowledge pack"})
		} else {
			seenRuleIDs[rule.ID] = struct{}{}
		}
		if strings.TrimSpace(rule.Version) == "" {
			problems = append(problems, Problem{Path: path + ".version", Message: "must not be empty"})
		}
		if len(rule.Conditions) == 0 {
			problems = append(problems, Problem{Path: path + ".conditions", Message: "must not be empty"})
		}
		if len(rule.Conclusions) == 0 {
			problems = append(problems, Problem{Path: path + ".conclusions", Message: "must not be empty"})
		}
		if err := validateKnowledgeConfidence(rule.BaseConfidence); err != nil {
			problems = append(problems, Problem{Path: path + ".base_confidence", Message: err.Error()})
		}
		if !validRiskLevel(rule.Risk) {
			problems = append(problems, Problem{Path: path + ".risk", Message: "must be low, medium, high, or critical"})
		}
		if rule.AutoApply {
			problems = append(problems, Problem{Path: path + ".auto_apply", Message: "must be false in knowledge-pack/v1"})
		}
		seenConflictKeys := map[string]struct{}{}
		for j, key := range rule.ConflictKeys {
			keyPath := fmt.Sprintf("%s.conflict_keys[%d]", path, j)
			key = strings.TrimSpace(key)
			if key == "" {
				problems = append(problems, Problem{Path: keyPath, Message: "must not be empty"})
				continue
			}
			if _, exists := seenConflictKeys[key]; exists {
				problems = append(problems, Problem{Path: keyPath, Message: "must be unique within a rule"})
				continue
			}
			seenConflictKeys[key] = struct{}{}
		}
		for j, condition := range rule.Conditions {
			problems = append(problems, validateCondition(condition, fmt.Sprintf("%s.conditions[%d]", path, j))...)
		}
		for j, conclusion := range rule.Conclusions {
			conclusionPath := fmt.Sprintf("%s.conclusions[%d]", path, j)
			if conclusion.Kind != "finding" {
				problems = append(problems, Problem{Path: conclusionPath + ".kind", Message: "must be finding in knowledge-pack/v1"})
			}
			if strings.TrimSpace(conclusion.Code) == "" {
				problems = append(problems, Problem{Path: conclusionPath + ".code", Message: "must not be empty"})
			}
			if !validSeverity(conclusion.Severity) {
				problems = append(problems, Problem{Path: conclusionPath + ".severity", Message: "must be info, warning, or error"})
			}
			if conclusion.Status != "" && !validFindingStatus(conclusion.Status) {
				problems = append(problems, Problem{Path: conclusionPath + ".status", Message: "must be confirmed, hypothesis, unknown, or conflict"})
			}
			if conclusion.Risk != "" && !validRiskLevel(conclusion.Risk) {
				problems = append(problems, Problem{Path: conclusionPath + ".risk", Message: "must be low, medium, high, or critical"})
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Slice(problems, func(i, j int) bool {
		if problems[i].Path == problems[j].Path {
			return problems[i].Message < problems[j].Message
		}
		return problems[i].Path < problems[j].Path
	})
	return &KnowledgeValidationError{Problems: problems}
}

func validateCondition(condition Condition, path string) []Problem {
	problems := make([]Problem, 0)
	if !validConditionOp(condition.Op) {
		return append(problems, Problem{Path: path + ".op", Message: "must be fact_exists, fact_state, string_equals, or string_in"})
	}
	switch condition.Op {
	case ConditionFactExists:
		problems = append(problems, forbiddenConditionFields(condition, path, true, true, true)...)
	case ConditionFactState:
		if !validTruthState(condition.State) {
			problems = append(problems, Problem{Path: path + ".state", Message: "must be known, unknown, or conflict"})
		}
		problems = append(problems, forbiddenConditionFields(condition, path, false, true, true)...)
	case ConditionStringEqual:
		if strings.TrimSpace(condition.Value) == "" {
			problems = append(problems, Problem{Path: path + ".value", Message: "must not be empty for string_equals"})
		}
		problems = append(problems, forbiddenConditionFields(condition, path, true, false, true)...)
	case ConditionStringIn:
		if len(condition.Values) == 0 {
			problems = append(problems, Problem{Path: path + ".values", Message: "must not be empty for string_in"})
		}
		for i, value := range condition.Values {
			if strings.TrimSpace(value) == "" {
				problems = append(problems, Problem{Path: fmt.Sprintf("%s.values[%d]", path, i), Message: "must not be empty"})
			}
		}
		problems = append(problems, forbiddenConditionFields(condition, path, true, true, false)...)
	}
	return problems
}

func forbiddenConditionFields(condition Condition, path string, state, value, values bool) []Problem {
	problems := make([]Problem, 0)
	if state && condition.State != "" {
		problems = append(problems, Problem{Path: path + ".state", Message: "is not allowed for this op"})
	}
	if value && condition.Value != "" {
		problems = append(problems, Problem{Path: path + ".value", Message: "is not allowed for this op"})
	}
	if values && len(condition.Values) != 0 {
		problems = append(problems, Problem{Path: path + ".values", Message: "is not allowed for this op"})
	}
	return problems
}

func validateKnowledgeConfidence(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return fmt.Errorf("must be in [0, 1]")
	}
	return nil
}

func validConditionOp(op ConditionOp) bool {
	return op == ConditionFactExists || op == ConditionFactState || op == ConditionStringEqual || op == ConditionStringIn
}

func validSeverity(severity string) bool {
	return severity == "info" || severity == "warning" || severity == "error"
}
