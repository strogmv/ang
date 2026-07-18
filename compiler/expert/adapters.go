package expert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/facts"
)

// AdaptFacts converts stable ang/facts/v1 observations into normalized expert
// facts. It does not infer negative facts: an omitted field is absent evidence,
// not proof that the corresponding property is missing.
func AdaptFacts(env facts.Envelope) ([]Fact, []Evidence, error) {
	if err := facts.Validate(env); err != nil {
		return nil, nil, err
	}
	result := adaptedFacts{byID: map[string]Fact{}, evidenceByID: map[string]Evidence{}}
	for _, entity := range env.Entities {
		result.add("entity", entity.Name, "exists", true, entity.Source, env)
		for _, field := range entity.Fields {
			result.add("field", entity.Name, field.Name, true, entity.Source, env)
		}
	}
	for _, operation := range env.Operations {
		subject := operationSubject(operation)
		result.add("operation", subject, "exists", true, operation.Source, env)
		for _, field := range operation.InputFields {
			result.add("operation_input", subject, field.Name, true, operation.Source, env)
		}
		for _, field := range operation.OutputFields {
			result.add("operation_output", subject, field.Name, true, operation.Source, env)
		}
	}
	for _, repository := range env.Repositories {
		for _, method := range repository.Methods {
			result.add("repository_method", repository.Entity, method.Name, method.Returns, repository.Source, env)
		}
	}
	for _, event := range env.Events {
		result.add("event", event.Name, "exists", true, event.Source, env)
		for _, field := range event.PayloadFields {
			result.add("event_field", event.Name, field.Name, true, event.Source, env)
		}
	}
	for _, endpoint := range env.Endpoints {
		subject := endpoint.Operation
		if subject == "" {
			continue
		}
		result.add("endpoint", subject, "exists", true, endpoint.Source, env)
		if strings.TrimSpace(endpoint.AuthExpr) != "" {
			result.add("endpoint", subject, "auth_expr", endpoint.AuthExpr, endpoint.Source, env)
		}
	}
	for _, rule := range env.SecurityRules {
		subject := strings.TrimSpace(rule.Scope)
		if subject == "" {
			subject = "global"
		}
		result.add("security_rule", subject, rule.Pattern, rule.Requirement, rule.Source, env)
	}
	return result.sorted(), result.sortedEvidence(), nil
}

type adaptedFacts struct {
	byID         map[string]Fact
	evidenceByID map[string]Evidence
}

func (a *adaptedFacts) add(kind, subject, predicate string, value any, source string, env facts.Envelope) {
	raw, err := json.Marshal(value)
	if err != nil {
		panic("expert: facts adapter value must be JSON-marshalable")
	}
	contentHash := stableHash(strings.Join([]string{kind, subject, predicate, string(raw)}, "\x00"))
	evidenceHash := stableHash(contentHash + "\x00" + stableEvidenceSource(firstNonEmpty(source, env.SourcePath)))
	evidenceID := "evidence.facts." + evidenceHash[:24]
	if _, exists := a.evidenceByID[evidenceID]; !exists {
		a.evidenceByID[evidenceID] = Evidence{
			ID: evidenceID, SourceType: env.SourceType, SourcePath: firstNonEmpty(source, env.SourcePath),
			Extractor: "ang/facts/v1", ContentHash: evidenceHash,
		}
	}
	factID := "fact.facts." + contentHash[:24]
	if existing, exists := a.byID[factID]; exists {
		existing.EvidenceIDs = sortedUnique(append(existing.EvidenceIDs, evidenceID))
		a.byID[factID] = existing
		return
	}
	a.byID[factID] = Fact{
		ID: factID, Kind: kind, Subject: subject, Predicate: predicate, Value: json.RawMessage(raw),
		State: TruthKnown, Confidence: 1, EvidenceIDs: []string{evidenceID},
	}
}

func stableEvidenceSource(source string) string {
	source = filepath.ToSlash(strings.TrimSpace(source))
	if filepath.IsAbs(source) {
		return filepath.Base(source)
	}
	return source
}

func (a *adaptedFacts) sorted() []Fact {
	out := make([]Fact, 0, len(a.byID))
	for _, fact := range a.byID {
		out = append(out, fact)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (a *adaptedFacts) sortedEvidence() []Evidence {
	out := make([]Evidence, 0, len(a.evidenceByID))
	for _, evidence := range a.evidenceByID {
		out = append(out, evidence)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func operationSubject(operation facts.Operation) string {
	if strings.TrimSpace(operation.ServiceHint) == "" {
		return operation.Name
	}
	return operation.ServiceHint + "." + operation.Name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stableHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
