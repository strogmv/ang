package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

const (
	codeMigrationFactsRequired      = "E_MIGRATION_FACTS_REQUIRED"
	codeMigrationFactsLoadFailed    = "E_MIGRATION_FACTS_LOAD_FAILED"
	codeMigrationGapWithoutQuestion = "E_MIGRATION_GAP_WITHOUT_OPEN_QUESTION"
	codeMigrationGapMarked          = "W_MIGRATION_GAP_MARKED"
)

var reqRefRe = regexp.MustCompile(`\breq\.([A-Za-z_][A-Za-z0-9_]*)\b`)

func loadFactsEnvelope(path string) (*FactsEnvelope, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var env FactsEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if strings.TrimSpace(env.Schema) == "" {
		env.Schema = "ang/facts/v1"
	}
	if env.Schema != "ang/facts/v1" {
		return nil, fmt.Errorf("unsupported facts schema %q (expected ang/facts/v1)", env.Schema)
	}
	return &env, nil
}

func runMigrationLints(services []normalizer.Service, facts *FactsEnvelope, profile lintProfile) []normalizer.Warning {
	if facts == nil {
		return nil
	}
	var out []normalizer.Warning
	for _, svc := range services {
		for _, method := range svc.Methods {
			opPath := svc.Name + "." + method.Name
			factOp, ok := findFactOp(facts.Operations, svc.Name, method.Name)
			if !ok {
				out = append(out, migrationGapWarning(
					opPath,
					method.Flow,
					nil,
					profile,
					"operation facts are missing",
					"No matching facts.operation found for this method. Add facts or mark unresolved parts with unknown.* / openQuestion.",
				))
				continue
			}

			inputSet := buildInputFieldSet(factOp)
			missing := findMissingReqRefs(method.Flow, inputSet)
			if len(missing) == 0 {
				continue
			}

			msgFields := make([]string, 0, len(missing))
			for _, m := range missing {
				msgFields = append(msgFields, m.Field)
			}
			sort.Strings(msgFields)
			hint := "Replace uncertain references with unknown.* or openQuestion, or update facts extraction so these input fields are present."
			message := fmt.Sprintf("req fields not found in facts for operation '%s': %s", factOp.Name, strings.Join(msgFields, ", "))
			out = append(out, migrationGapWarning(opPath, method.Flow, missing, profile, message, hint))
		}
	}
	return out
}

type missingReqRef struct {
	Field   string
	Step    int
	CUEPath string
	File    string
	Line    int
	Column  int
}

func migrationGapWarning(opPath string, flow []normalizer.FlowStep, missing []missingReqRef, profile lintProfile, message string, hint string) normalizer.Warning {
	hasMarker := operationHasQuestionMarker(flow)
	code := codeMigrationGapWithoutQuestion
	sev := "error"
	if hasMarker {
		code = codeMigrationGapMarked
		sev = "warn"
	}

	w := normalizer.Warning{
		Kind:     "migration",
		Code:     code,
		Severity: severityForProfile(code, profile, sev),
		Message:  message,
		Op:       opPath,
		CUEPath:  opPath + ".flow",
		Hint:     hint,
	}
	if len(missing) > 0 {
		w.Step = missing[0].Step
		w.File = missing[0].File
		w.Line = missing[0].Line
		w.Column = missing[0].Column
		if strings.TrimSpace(missing[0].CUEPath) != "" {
			w.CUEPath = missing[0].CUEPath
		}
	}
	return w
}

func findFactOp(ops []FactOp, serviceName, methodName string) (FactOp, bool) {
	methodNorm := normalizeID(methodName)
	serviceNorm := normalizeID(serviceName)
	var candidates []FactOp
	for _, op := range ops {
		if normalizeID(op.Name) == methodNorm {
			candidates = append(candidates, op)
		}
	}
	if len(candidates) == 0 {
		return FactOp{}, false
	}
	for _, c := range candidates {
		if normalizeID(c.ServiceHint) == serviceNorm {
			return c, true
		}
	}
	return candidates[0], true
}

func buildInputFieldSet(op FactOp) map[string]struct{} {
	set := make(map[string]struct{}, len(op.InputFields))
	for _, f := range op.InputFields {
		norm := normalizeID(f.Name)
		if norm != "" {
			set[norm] = struct{}{}
		}
	}
	return set
}

func findMissingReqRefs(flow []normalizer.FlowStep, inputSet map[string]struct{}) []missingReqRef {
	seen := make(map[string]struct{})
	var out []missingReqRef
	walkFlow(flow, false, false, func(step normalizer.FlowStep, i int, inTx bool, inTimeout bool) {
		for _, expr := range stepExpressions(step.Args) {
			matches := reqRefRe.FindAllStringSubmatch(expr, -1)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				field := strings.TrimSpace(m[1])
				norm := normalizeID(field)
				if norm == "" {
					continue
				}
				if _, ok := inputSet[norm]; ok {
					continue
				}
				if _, dup := seen[norm]; dup {
					continue
				}
				seen[norm] = struct{}{}
				out = append(out, missingReqRef{
					Field:   field,
					Step:    i,
					CUEPath: step.CUEPath,
					File:    step.File,
					Line:    step.Line,
					Column:  step.Column,
				})
			}
		}
	})
	return out
}

func operationHasQuestionMarker(flow []normalizer.FlowStep) bool {
	has := false
	walkFlow(flow, false, false, func(step normalizer.FlowStep, i int, inTx bool, inTimeout bool) {
		if has {
			return
		}
		for _, expr := range stepExpressions(step.Args) {
			s := strings.ToLower(strings.TrimSpace(expr))
			if strings.Contains(s, "unknown.") || strings.Contains(s, "openquestion") {
				has = true
				return
			}
		}
	})
	return has
}

func stepExpressions(args map[string]any) []string {
	var out []string
	for key, raw := range args {
		if strings.HasPrefix(key, "_") {
			continue
		}
		collectArgStrings(raw, &out)
	}
	return out
}

func collectArgStrings(v any, out *[]string) {
	switch x := v.(type) {
	case string:
		*out = append(*out, x)
	case []string:
		*out = append(*out, x...)
	case []any:
		for _, item := range x {
			collectArgStrings(item, out)
		}
	case map[string]any:
		for _, val := range x {
			collectArgStrings(val, out)
		}
	case map[string]string:
		for _, val := range x {
			collectArgStrings(val, out)
		}
	}
}

func normalizeID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
