package facts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// Canonicalize returns a deterministically ordered copy of env. It never
// mutates caller-owned slices. Lists whose order carries source semantics
// (enum values, operation calls, evidence) deliberately retain their order.
func Canonicalize(env Envelope) Envelope {
	out := env
	out.Entities = append([]Entity(nil), env.Entities...)
	for i := range out.Entities {
		out.Entities[i].Fields = canonicalFields(out.Entities[i].Fields)
	}
	sort.SliceStable(out.Entities, func(i, j int) bool {
		return compare(out.Entities[i].Name, out.Entities[j].Name) < 0
	})

	out.Operations = append([]Operation(nil), env.Operations...)
	for i := range out.Operations {
		out.Operations[i].InputFields = canonicalFields(out.Operations[i].InputFields)
		out.Operations[i].OutputFields = canonicalFields(out.Operations[i].OutputFields)
	}
	sort.SliceStable(out.Operations, func(i, j int) bool {
		return operationKey(out.Operations[i]) < operationKey(out.Operations[j])
	})

	out.Repositories = append([]Repository(nil), env.Repositories...)
	for i := range out.Repositories {
		out.Repositories[i].Methods = append([]RepositoryMethod(nil), out.Repositories[i].Methods...)
		sort.SliceStable(out.Repositories[i].Methods, func(a, b int) bool {
			return compare(out.Repositories[i].Methods[a].Name, out.Repositories[i].Methods[b].Name) < 0
		})
	}
	sort.SliceStable(out.Repositories, func(i, j int) bool {
		return compare(out.Repositories[i].Entity, out.Repositories[j].Entity) < 0
	})

	out.Events = append([]Event(nil), env.Events...)
	for i := range out.Events {
		out.Events[i].PayloadFields = canonicalFields(out.Events[i].PayloadFields)
	}
	sort.SliceStable(out.Events, func(i, j int) bool {
		return compare(out.Events[i].Name, out.Events[j].Name) < 0
	})

	out.Constants = append([]Constant(nil), env.Constants...)
	sort.SliceStable(out.Constants, func(i, j int) bool { return compare(out.Constants[i].Name, out.Constants[j].Name) < 0 })
	out.Enums = append([]Enum(nil), env.Enums...)
	sort.SliceStable(out.Enums, func(i, j int) bool { return compare(out.Enums[i].Name, out.Enums[j].Name) < 0 })
	out.Endpoints = append([]Endpoint(nil), env.Endpoints...)
	sort.SliceStable(out.Endpoints, func(i, j int) bool { return endpointKey(out.Endpoints[i]) < endpointKey(out.Endpoints[j]) })
	out.Calls = append([]CallEdge(nil), env.Calls...)
	sort.SliceStable(out.Calls, func(i, j int) bool { return callKey(out.Calls[i]) < callKey(out.Calls[j]) })
	out.Mappers = append([]Mapper(nil), env.Mappers...)
	for i := range out.Mappers {
		out.Mappers[i].Methods = append([]MapperMethod(nil), out.Mappers[i].Methods...)
		sort.SliceStable(out.Mappers[i].Methods, func(a, b int) bool {
			return compare(out.Mappers[i].Methods[a].Name, out.Mappers[i].Methods[b].Name) < 0
		})
	}
	sort.SliceStable(out.Mappers, func(i, j int) bool { return compare(out.Mappers[i].Name, out.Mappers[j].Name) < 0 })
	out.ErrorContracts = append([]ErrorContract(nil), env.ErrorContracts...)
	sort.SliceStable(out.ErrorContracts, func(i, j int) bool {
		return compare(out.ErrorContracts[i].Exception, out.ErrorContracts[j].Exception) < 0
	})
	out.SecurityRules = append([]SecurityRule(nil), env.SecurityRules...)
	sort.SliceStable(out.SecurityRules, func(i, j int) bool {
		return securityRuleKey(out.SecurityRules[i]) < securityRuleKey(out.SecurityRules[j])
	})
	return out
}

// CanonicalJSON returns the stable JSON representation used for facts hashes.
func CanonicalJSON(env Envelope) ([]byte, error) {
	if err := Validate(env); err != nil {
		return nil, err
	}
	return json.Marshal(Canonicalize(env))
}

// Hash returns the lowercase SHA-256 of CanonicalJSON(env).
func Hash(env Envelope) (string, error) {
	data, err := CanonicalJSON(env)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalFields(fields []Field) []Field {
	out := append([]Field(nil), fields...)
	sort.SliceStable(out, func(i, j int) bool { return compare(out[i].Name, out[j].Name) < 0 })
	return out
}

func operationKey(operation Operation) string {
	return strings.ToLower(strings.TrimSpace(operation.ServiceHint)) + "\x00" + strings.ToLower(strings.TrimSpace(operation.Name))
}

func endpointKey(endpoint Endpoint) string {
	return strings.ToUpper(strings.TrimSpace(endpoint.HTTPMethod)) + "\x00" + strings.TrimSpace(endpoint.HTTPPath) + "\x00" + strings.ToLower(strings.TrimSpace(endpoint.Operation))
}

func callKey(call CallEdge) string {
	return strings.ToLower(strings.TrimSpace(call.From)) + "\x00" + strings.ToLower(strings.TrimSpace(call.To)) + "\x00" + strings.ToLower(strings.TrimSpace(call.Kind))
}

func securityRuleKey(rule SecurityRule) string {
	return strings.ToLower(strings.TrimSpace(rule.Scope)) + "\x00" + strings.TrimSpace(rule.Pattern) + "\x00" + strings.TrimSpace(rule.Requirement)
}

func compare(left, right string) int {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
