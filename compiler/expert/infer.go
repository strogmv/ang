package expert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type InferenceResult struct {
	Findings []Finding
	Trace    []RuleTrace
}

type matchedRule struct {
	rule           Rule
	traceIndex     int
	findingIndexes []int
}

type conditionResult string

const (
	conditionMatched    conditionResult = "matched"
	conditionNotMatched conditionResult = "not_matched"
	conditionUnknown    conditionResult = "unknown"
	conditionConflict   conditionResult = "conflict"
)

// Infer evaluates one validated knowledge pack against normalized facts. It is
// read-only: v1 conclusions produce findings only, never patches or proposals.
func Infer(facts []Fact, pack KnowledgePack) (InferenceResult, error) {
	if err := ValidateKnowledgePack(pack); err != nil {
		return InferenceResult{}, err
	}
	for i, fact := range facts {
		if err := ValidateFact(fact); err != nil {
			return InferenceResult{}, fmt.Errorf("facts[%d]: %w", i, err)
		}
	}
	rules := append([]Rule(nil), pack.Rules...)
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Priority == rules[j].Priority {
			return rules[i].ID < rules[j].ID
		}
		return rules[i].Priority > rules[j].Priority
	})

	result := InferenceResult{Findings: []Finding{}, Trace: []RuleTrace{}}
	seenFindings := map[string]struct{}{}
	matchedRules := make([]matchedRule, 0)
	for _, rule := range rules {
		outcome, matchedFacts, missingFacts := evaluateRule(rule, facts)
		trace := RuleTrace{
			Origin: "knowledge", RuleID: rule.ID, Result: string(outcome),
			MatchedFacts: matchedFacts, MissingFacts: missingFacts,
		}
		if outcome == conditionConflict {
			factIDs, evidenceIDs, confidence := matchedFactMetadata(rule, facts, matchedFacts)
			finding := factConflictFinding(pack, rule, factIDs, evidenceIDs, confidence)
			if _, exists := seenFindings[finding.ID]; !exists {
				seenFindings[finding.ID] = struct{}{}
				result.Findings = append(result.Findings, finding)
				trace.ProducedIDs = append(trace.ProducedIDs, finding.ID)
			}
			result.Trace = append(result.Trace, trace)
			continue
		}
		if outcome != conditionMatched {
			result.Trace = append(result.Trace, trace)
			continue
		}
		factIDs, evidenceIDs, confidence := matchedFactMetadata(rule, facts, matchedFacts)
		matched := matchedRule{rule: rule, traceIndex: len(result.Trace)}
		for _, conclusion := range rule.Conclusions {
			finding := Finding{
				ID:          inferredFindingID(pack, rule, conclusion, factIDs),
				Code:        conclusion.Code,
				Severity:    conclusion.Severity,
				Summary:     conclusion.Summary,
				Origin:      "knowledge",
				RuleID:      rule.ID,
				FactIDs:     factIDs,
				EvidenceIDs: evidenceIDs,
				Confidence:  confidence,
				Status:      defaultFindingStatus(conclusion.Status),
			}
			if _, exists := seenFindings[finding.ID]; exists {
				continue
			}
			seenFindings[finding.ID] = struct{}{}
			result.Findings = append(result.Findings, finding)
			matched.findingIndexes = append(matched.findingIndexes, len(result.Findings)-1)
			trace.ProducedIDs = append(trace.ProducedIDs, finding.ID)
		}
		result.Trace = append(result.Trace, trace)
		matchedRules = append(matchedRules, matched)
	}
	applyRuleConflicts(pack, &result, matchedRules, seenFindings)
	sort.SliceStable(result.Findings, func(i, j int) bool { return result.Findings[i].ID < result.Findings[j].ID })
	sort.SliceStable(result.Trace, func(i, j int) bool { return result.Trace[i].RuleID < result.Trace[j].RuleID })
	return result, nil
}

func applyRuleConflicts(pack KnowledgePack, result *InferenceResult, matchedRules []matchedRule, seenFindings map[string]struct{}) {
	byKey := make(map[string][]matchedRule)
	for _, matched := range matchedRules {
		for _, key := range sortedUnique(matched.rule.ConflictKeys) {
			byKey[key] = append(byKey[key], matched)
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		conflicting := byKey[key]
		if len(conflicting) < 2 {
			continue
		}
		ruleIDs, factIDs, evidenceIDs := make([]string, 0), make([]string, 0), make([]string, 0)
		confidence := 1.0
		for _, matched := range conflicting {
			ruleIDs = append(ruleIDs, matched.rule.ID)
			for _, index := range matched.findingIndexes {
				finding := &result.Findings[index]
				finding.Status = FindingConflict
				factIDs = append(factIDs, finding.FactIDs...)
				evidenceIDs = append(evidenceIDs, finding.EvidenceIDs...)
				if finding.Confidence < confidence {
					confidence = finding.Confidence
				}
			}
		}
		ruleIDs = sortedUnique(ruleIDs)
		factIDs = sortedUnique(factIDs)
		evidenceIDs = sortedUnique(evidenceIDs)
		conflict := ruleConflictFinding(pack, key, ruleIDs, factIDs, evidenceIDs, confidence)
		if _, exists := seenFindings[conflict.ID]; exists {
			continue
		}
		seenFindings[conflict.ID] = struct{}{}
		result.Findings = append(result.Findings, conflict)
		reason := fmt.Sprintf("conflict key %q matched mutually exclusive rules: %s", key, strings.Join(ruleIDs, ", "))
		for _, matched := range conflicting {
			trace := &result.Trace[matched.traceIndex]
			trace.Result = string(conditionConflict)
			trace.RejectedReason = reason
			trace.ProducedIDs = sortedUnique(append(trace.ProducedIDs, conflict.ID))
		}
	}
}

func evaluateRule(rule Rule, facts []Fact) (conditionResult, []string, []string) {
	missingRequiredKinds := make([]string, 0)
	for _, kind := range rule.RequiredKinds {
		found := false
		for _, fact := range facts {
			if fact.Kind == kind {
				found = true
				break
			}
		}
		if !found {
			missingRequiredKinds = append(missingRequiredKinds, "kind="+kind)
		}
	}
	if len(missingRequiredKinds) > 0 {
		return conditionUnknown, nil, sortedUnique(missingRequiredKinds)
	}
	matchedIDs := make([]string, 0)
	missing := make([]string, 0)
	hasUnknown := false
	hasConflict := false
	for _, condition := range rule.Conditions {
		outcome, ids, missingRef := evaluateCondition(condition, facts)
		matchedIDs = append(matchedIDs, ids...)
		if missingRef != "" {
			missing = append(missing, missingRef)
		}
		switch outcome {
		case conditionNotMatched:
			return conditionNotMatched, sortedUnique(matchedIDs), sortedUnique(missing)
		case conditionConflict:
			hasConflict = true
		case conditionUnknown:
			hasUnknown = true
		}
	}
	if hasConflict {
		return conditionConflict, sortedUnique(matchedIDs), sortedUnique(missing)
	}
	if hasUnknown {
		return conditionUnknown, sortedUnique(matchedIDs), sortedUnique(missing)
	}
	return conditionMatched, sortedUnique(matchedIDs), sortedUnique(missing)
}

func evaluateCondition(condition Condition, facts []Fact) (conditionResult, []string, string) {
	candidates := matchingFacts(condition, facts)
	if len(candidates) == 0 {
		return conditionUnknown, nil, conditionReference(condition)
	}
	if condition.Op == ConditionFactState {
		matchedIDs := make([]string, 0, len(candidates))
		hasUnknown := false
		hasConflict := false
		for _, fact := range candidates {
			if fact.State == condition.State {
				matchedIDs = append(matchedIDs, fact.ID)
			}
			switch fact.State {
			case TruthConflict:
				if condition.State != TruthConflict {
					hasConflict = true
					matchedIDs = append(matchedIDs, fact.ID)
				}
			case TruthUnknown:
				if condition.State != TruthUnknown {
					hasUnknown = true
				}
			}
		}
		if hasConflict {
			return conditionConflict, sortedUnique(matchedIDs), ""
		}
		if len(matchedIDs) > 0 {
			return conditionMatched, sortedUnique(matchedIDs), ""
		}
		if hasUnknown {
			return conditionUnknown, nil, conditionReference(condition)
		}
		return conditionNotMatched, nil, ""
	}
	matchedIDs := make([]string, 0, len(candidates))
	hasUnknown := false
	hasConflict := false
	hasKnown := false
	for _, fact := range candidates {
		switch fact.State {
		case TruthConflict:
			hasConflict = true
			matchedIDs = append(matchedIDs, fact.ID)
		case TruthUnknown:
			hasUnknown = true
		case TruthKnown:
			hasKnown = true
			if conditionMatchesKnownFact(condition, fact) {
				matchedIDs = append(matchedIDs, fact.ID)
			}
		}
	}
	if hasConflict {
		return conditionConflict, matchedIDs, ""
	}
	if len(matchedIDs) > 0 {
		return conditionMatched, matchedIDs, ""
	}
	if hasUnknown {
		return conditionUnknown, nil, conditionReference(condition)
	}
	if hasKnown {
		return conditionNotMatched, nil, ""
	}
	return conditionUnknown, nil, conditionReference(condition)
}

func matchingFacts(condition Condition, facts []Fact) []Fact {
	out := make([]Fact, 0)
	for _, fact := range facts {
		if condition.FactKind != "" && fact.Kind != condition.FactKind {
			continue
		}
		if condition.Subject != "" && fact.Subject != condition.Subject {
			continue
		}
		if condition.Predicate != "" && fact.Predicate != condition.Predicate {
			continue
		}
		out = append(out, fact)
	}
	return out
}

func conditionMatchesKnownFact(condition Condition, fact Fact) bool {
	switch condition.Op {
	case ConditionFactExists:
		return true
	case ConditionFactState:
		return fact.State == condition.State
	case ConditionStringEqual, ConditionStringIn:
		var value string
		if err := json.Unmarshal(fact.Value, &value); err != nil {
			return false
		}
		if condition.Op == ConditionStringEqual {
			return value == condition.Value
		}
		for _, candidate := range condition.Values {
			if value == candidate {
				return true
			}
		}
	}
	return false
}

func matchedFactMetadata(rule Rule, facts []Fact, matchedIDs []string) ([]string, []string, float64) {
	matched := make(map[string]Fact, len(facts))
	for _, fact := range facts {
		matched[fact.ID] = fact
	}
	factIDs := sortedUnique(matchedIDs)
	evidenceIDs := make([]string, 0)
	confidence := rule.BaseConfidence
	for _, id := range factIDs {
		fact := matched[id]
		if fact.Confidence < confidence {
			confidence = fact.Confidence
		}
		evidenceIDs = append(evidenceIDs, fact.EvidenceIDs...)
	}
	return factIDs, sortedUnique(evidenceIDs), confidence
}

func inferredFindingID(pack KnowledgePack, rule Rule, conclusion Conclusion, factIDs []string) string {
	content := strings.Join([]string{pack.Name, pack.Version, rule.ID, conclusion.Code, strings.Join(factIDs, ",")}, "\x00")
	sum := sha256.Sum256([]byte(content))
	return "finding.knowledge." + hex.EncodeToString(sum[:12])
}

func factConflictFinding(pack KnowledgePack, rule Rule, factIDs, evidenceIDs []string, confidence float64) Finding {
	content := strings.Join([]string{pack.Name, pack.Version, rule.ID, strings.Join(factIDs, ",")}, "\x00")
	sum := sha256.Sum256([]byte(content))
	return Finding{
		ID:          "finding.knowledge.conflict." + hex.EncodeToString(sum[:12]),
		Code:        "EXPERT_FACT_CONFLICT",
		Severity:    "warning",
		Summary:     fmt.Sprintf("Rule %q could not be evaluated because its source facts conflict.", rule.ID),
		Origin:      "knowledge",
		RuleID:      rule.ID,
		FactIDs:     factIDs,
		EvidenceIDs: evidenceIDs,
		Confidence:  confidence,
		Status:      FindingConflict,
	}
}

func ruleConflictFinding(pack KnowledgePack, key string, ruleIDs, factIDs, evidenceIDs []string, confidence float64) Finding {
	content := strings.Join([]string{pack.Name, pack.Version, key, strings.Join(ruleIDs, ","), strings.Join(factIDs, ",")}, "\x00")
	sum := sha256.Sum256([]byte(content))
	return Finding{
		ID:          "finding.knowledge.conflict." + hex.EncodeToString(sum[:12]),
		Code:        "EXPERT_RULE_CONFLICT",
		Severity:    "warning",
		Summary:     fmt.Sprintf("Mutually exclusive rules matched conflict key %q: %s.", key, strings.Join(ruleIDs, ", ")),
		Origin:      "knowledge",
		RuleID:      "conflict_key." + key,
		FactIDs:     factIDs,
		EvidenceIDs: evidenceIDs,
		Confidence:  confidence,
		Status:      FindingConflict,
	}
}

func defaultFindingStatus(status FindingStatus) FindingStatus {
	if status == "" {
		return FindingConfirmed
	}
	return status
}

func conditionReference(condition Condition) string {
	parts := []string{string(condition.Op)}
	if condition.FactKind != "" {
		parts = append(parts, "kind="+condition.FactKind)
	}
	if condition.Subject != "" {
		parts = append(parts, "subject="+condition.Subject)
	}
	if condition.Predicate != "" {
		parts = append(parts, "predicate="+condition.Predicate)
	}
	return strings.Join(parts, " ")
}

func sortedUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := append([]string(nil), values...)
	sort.Strings(out)
	write := 1
	for read := 1; read < len(out); read++ {
		if out[read] == out[write-1] {
			continue
		}
		out[write] = out[read]
		write++
	}
	return out[:write]
}
