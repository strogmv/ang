package ppfacts

import (
	"fmt"
	"regexp"
	"strings"
)

const maxTermValueLen = 2048

var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var knownPredicates = map[string]struct{}{
	"pp_provider":       {},
	"pp_capability":     {},
	"pp_operation":      {},
	"pp_schema_sync":    {},
	"pp_schema_drift":   {},
	"pp_vet_issue":      {},
	"pp_secret_part":    {},
	"pp_auth":           {},
	"pp_endpoint":       {},
	"pp_runtime_policy": {},
	"pp_behavior":       {},
	"pp_test_area":      {},
}

var predicateArity = map[string]int{
	"pp_provider":       2,
	"pp_capability":     3,
	"pp_operation":      4,
	"pp_schema_sync":    2,
	"pp_schema_drift":   2,
	"pp_vet_issue":      3,
	"pp_secret_part":    4,
	"pp_auth":           4,
	"pp_endpoint":       4,
	"pp_runtime_policy": 3,
	"pp_behavior":       2,
	"pp_test_area":      3,
}

var validSeverities = map[string]struct{}{
	"info":    {},
	"warning": {},
	"error":   {},
}

// Validate checks envelope against ang/payment-provider-facts/v1 rules.
func Validate(env Envelope) error {
	if env.Schema != SchemaV1 {
		return fmt.Errorf("facts.schema must equal %q", SchemaV1)
	}
	if strings.TrimSpace(env.ScopeID) == "" {
		return fmt.Errorf("facts.scope_id must not be empty")
	}
	if strings.TrimSpace(env.ProviderID) == "" {
		return fmt.Errorf("facts.provider_id must not be empty")
	}
	factIDs := map[string]struct{}{}
	for i, fact := range env.Facts {
		if err := validateFact(fact, i); err != nil {
			return err
		}
		if _, ok := factIDs[fact.ID]; ok {
			return fmt.Errorf("facts[%d].id duplicate %q", i, fact.ID)
		}
		factIDs[fact.ID] = struct{}{}
	}
	evidenceIDs := map[string]struct{}{}
	for i, evidence := range env.Evidence {
		if err := validateEvidence(evidence, i); err != nil {
			return err
		}
		if _, ok := evidenceIDs[evidence.ID]; ok {
			return fmt.Errorf("evidence[%d].id duplicate %q", i, evidence.ID)
		}
		evidenceIDs[evidence.ID] = struct{}{}
	}
	for i, fact := range env.Facts {
		for j, evidenceID := range fact.EvidenceIDs {
			if _, ok := evidenceIDs[evidenceID]; !ok {
				return fmt.Errorf("facts[%d].evidence_ids[%d] references missing evidence %q", i, j, evidenceID)
			}
		}
	}
	for i, diagnostic := range env.Diagnostics {
		if err := validateDiagnostic(diagnostic, i); err != nil {
			return err
		}
	}
	return nil
}

func validateFact(fact Fact, index int) error {
	if strings.TrimSpace(fact.ID) == "" {
		return fmt.Errorf("facts[%d].id must not be empty", index)
	}
	if _, ok := knownPredicates[fact.Predicate]; !ok {
		return fmt.Errorf("facts[%d].predicate %q is unknown", index, fact.Predicate)
	}
	wantArity := predicateArity[fact.Predicate]
	if len(fact.Terms) != wantArity {
		return fmt.Errorf("facts[%d].predicate %q requires %d terms", index, fact.Predicate, wantArity)
	}
	for j, term := range fact.Terms {
		if err := validateTerm(term, index, j); err != nil {
			return err
		}
	}
	return validateUniqueEvidenceIDs(fact.EvidenceIDs, index)
}

func validateTerm(term Term, factIndex, termIndex int) error {
	if strings.TrimSpace(term.Sort) == "" {
		return fmt.Errorf("facts[%d].terms[%d].sort must not be empty", factIndex, termIndex)
	}
	if strings.TrimSpace(term.Value) == "" {
		return fmt.Errorf("facts[%d].terms[%d].value must not be empty", factIndex, termIndex)
	}
	if strings.ContainsAny(term.Value, "\r\n") {
		return fmt.Errorf("facts[%d].terms[%d].value must not contain CR/LF", factIndex, termIndex)
	}
	if len(term.Value) > maxTermValueLen {
		return fmt.Errorf("facts[%d].terms[%d].value exceeds %d bytes", factIndex, termIndex, maxTermValueLen)
	}
	return nil
}

func validateEvidence(evidence Evidence, index int) error {
	if strings.TrimSpace(evidence.ID) == "" {
		return fmt.Errorf("evidence[%d].id must not be empty", index)
	}
	if strings.TrimSpace(evidence.Extractor) == "" {
		return fmt.Errorf("evidence[%d].extractor must not be empty", index)
	}
	if !hashPattern.MatchString(evidence.ContentHash) {
		return fmt.Errorf("evidence[%d].content_hash must be lowercase sha256 hex", index)
	}
	return nil
}

func validateDiagnostic(diagnostic Diagnostic, index int) error {
	if strings.TrimSpace(diagnostic.Code) == "" {
		return fmt.Errorf("diagnostics[%d].code must not be empty", index)
	}
	if strings.ContainsAny(diagnostic.Code, " \t\r\n") {
		return fmt.Errorf("diagnostics[%d].code must not contain whitespace", index)
	}
	if _, ok := validSeverities[diagnostic.Severity]; !ok {
		return fmt.Errorf("diagnostics[%d].severity is invalid", index)
	}
	return nil
}

func validateUniqueEvidenceIDs(ids []string, factIndex int) error {
	seen := map[string]struct{}{}
	for j, id := range ids {
		if _, ok := seen[id]; ok {
			return fmt.Errorf("facts[%d].evidence_ids[%d] duplicate evidence id", factIndex, j)
		}
		seen[id] = struct{}{}
	}
	return nil
}
