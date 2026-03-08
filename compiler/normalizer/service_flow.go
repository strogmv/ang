package normalizer

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
)

func (n *Normalizer) parseFlowSteps(val cue.Value) ([]FlowStep, error) {
	steps, err := n.rawParseFlowSteps(val)
	if err != nil {
		return nil, err
	}
	steps = n.autoCompleteFlowSteps(steps)
	steps = n.applyFlowPerformanceDefaults(steps)
	steps = n.resolvePolicyChecks(steps)
	return steps, nil
}

func (n *Normalizer) resolvePolicyChecks(steps []FlowStep) []FlowStep {
	if len(steps) == 0 {
		return steps
	}

	rewriteList := func(in []FlowStep) []FlowStep { return n.resolvePolicyChecks(in) }
	childKeys := []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"}

	for i := range steps {
		step := &steps[i]

		if step.Action == "policy.Check" {
			policyRaw, _ := step.Args["policy"].(string)
			policyName := strings.TrimSpace(policyRaw)
			if u, err := strconv.Unquote(policyName); err == nil {
				policyName = strings.TrimSpace(u)
			}
			if p, ok := n.Policies[policyName]; ok {
				step.Args["policy"] = strconv.Quote(policyName)
				step.Args["_policyResolved"] = true
				step.Args["_policyRoles"] = append([]string{}, p.Roles...)
				step.Args["_policySameCompany"] = p.SameCompany
				step.Args["_policyAllowAdminOverride"] = p.AllowAdminOverride
			}
		}

		for _, key := range childKeys {
			if child, ok := step.Args[key].([]FlowStep); ok {
				step.Args[key] = rewriteList(child)
			}
		}
		if cases, ok := step.Args["_cases"].(map[string][]FlowStep); ok {
			updated := make(map[string][]FlowStep, len(cases))
			for k, branch := range cases {
				updated[k] = rewriteList(branch)
			}
			step.Args["_cases"] = updated
		}
		if branches, ok := step.Args["_branches"].(map[string][]FlowStep); ok {
			updated := make(map[string][]FlowStep, len(branches))
			for k, branch := range branches {
				updated[k] = rewriteList(branch)
			}
			step.Args["_branches"] = updated
		}
	}
	return steps
}

// rawParseFlowSteps parses flow steps without auto-completion
func (n *Normalizer) rawParseFlowSteps(val cue.Value) ([]FlowStep, error) {
	var steps []FlowStep

	list, err := val.List()
	if err != nil {
		return nil, err
	}

	for list.Next() {
		stepVal := list.Value()
		action, _ := stepVal.LookupPath(cue.ParsePath("action")).String()
		if action == "" {
			continue
		}

		file := ""
		line := 0
		column := 0
		if pos := stepVal.Pos(); pos.IsValid() {
			file = pos.Filename()
			line = pos.Line()
			column = pos.Column()
			if file != "" {
				if cwd, err := os.Getwd(); err == nil {
					if rel, err := filepath.Rel(cwd, file); err == nil && !strings.HasPrefix(rel, "..") {
						file = rel
					}
				}
			}
		}
		step := FlowStep{
			Action:     action,
			Args:       make(map[string]any),
			File:       file,
			Line:       line,
			Column:     column,
			CUEPath:    stepVal.Path().String(),
			Attributes: parseAttributes(stepVal),
		}

		// Iterate over ALL fields
		it, _ := stepVal.Fields(cue.All())
		for it.Next() {
			label := it.Selector().String()

			// Skip recursion fields and internal definitions
			if label == "action" || label == "then" || label == "else" || label == "do" || label == "ifNew" || label == "ifExists" || label == "cases" || label == "default" || strings.HasPrefix(label, "#") {
				continue
			}

			v := it.Value()
			if action == "entity.PatchValidated" && label == "fields" && v.IncompleteKind() == cue.StructKind {
				fieldsMap := make(map[string]map[string]string)
				fit, _ := v.Fields(cue.All())
				for fit.Next() {
					fieldName := strings.Trim(fit.Selector().String(), "\"")
					fieldRules := make(map[string]string)
					fieldVal := fit.Value()
					if fieldVal.IncompleteKind() != cue.StructKind {
						continue
					}
					rit, _ := fieldVal.Fields(cue.All())
					for rit.Next() {
						ruleName := strings.Trim(rit.Selector().String(), "\"")
						ruleVal := rit.Value()
						switch ruleVal.Kind() {
						case cue.StringKind:
							if s, err := ruleVal.String(); err == nil {
								fieldRules[ruleName] = s
							}
						case cue.BoolKind:
							if b, err := ruleVal.Bool(); err == nil {
								fieldRules[ruleName] = strconv.FormatBool(b)
							}
						}
					}
					if len(fieldRules) > 0 {
						fieldsMap[fieldName] = fieldRules
					}
				}
				if len(fieldsMap) > 0 {
					step.Args["fields"] = fieldsMap
				}
				continue
			}
			if !v.IsConcrete() && v.Kind() != cue.ListKind {
				continue
			}

			switch v.Kind() {
			case cue.StringKind:
				if s, err := v.String(); err == nil {
					step.Args[label] = s
					if strings.HasPrefix(label, "_") {
						step.Args[strings.TrimPrefix(label, "_")] = s
					}
				}
			case cue.IntKind:
				if i, err := v.Int64(); err == nil {
					step.Args[label] = int(i)
					if strings.HasPrefix(label, "_") {
						step.Args[strings.TrimPrefix(label, "_")] = int(i)
					}
				}
			case cue.FloatKind, cue.NumberKind:
				if f, err := v.Float64(); err == nil {
					if f == float64(int64(f)) {
						step.Args[label] = int(f)
						if strings.HasPrefix(label, "_") {
							step.Args[strings.TrimPrefix(label, "_")] = int(f)
						}
					} else {
						step.Args[label] = f
						if strings.HasPrefix(label, "_") {
							step.Args[strings.TrimPrefix(label, "_")] = f
						}
					}
				}
			case cue.BoolKind:
				if b, err := v.Bool(); err == nil {
					step.Args[label] = b
					if strings.HasPrefix(label, "_") {
						step.Args[strings.TrimPrefix(label, "_")] = b
					}
				}
			case cue.ListKind:
				var p []string
				l, _ := v.List()
				for l.Next() {
					s, err := l.Value().String()
					if err == nil {
						p = append(p, s)
					} else {
						p = append(p, fmt.Sprintf("%v", l.Value()))
					}
				}
				if label == "params" {
					step.Params = p
				} else {
					step.Args[label] = p
					if strings.HasPrefix(label, "_") {
						step.Args[strings.TrimPrefix(label, "_")] = p
					}
				}
			case cue.StructKind:
				// Decode string-valued structs as map[string]string (e.g. headers, query).
				// Structs with non-string values are skipped (handled separately or unsupported).
				m := make(map[string]string)
				allStrings := true
				sit, _ := v.Fields(cue.All())
				for sit.Next() {
					sv := sit.Value()
					if sv.Kind() == cue.StringKind {
						if s, err := sv.String(); err == nil {
							m[strings.Trim(sit.Selector().String(), "\"")] = s
						}
					} else {
						allStrings = false
						break
					}
				}
				if allStrings && len(m) > 0 {
					step.Args[label] = m
					if strings.HasPrefix(label, "_") {
						step.Args[strings.TrimPrefix(label, "_")] = m
					}
				}
			}
		}

		// Explicit lookup for bool flags that may be skipped if not concrete
		for _, boolLabel := range []string{"ignoreErr", "failOnError"} {
			if _, ok := step.Args[boolLabel]; !ok {
				bv := stepVal.LookupPath(cue.ParsePath(boolLabel))
				if bv.Exists() {
					if b, err := bv.Bool(); err == nil {
						step.Args[boolLabel] = b
					}
				}
			}
		}
		// Explicit lookup for string fields that may be skipped if disjunction prevents IsConcrete()
		// Use Unify with a concrete empty string to force resolution, then check via JSON marshal
		for _, strLabel := range []string{"status", "method", "error", "algo"} {
			if _, ok := step.Args[strLabel]; !ok {
				sv := stepVal.LookupPath(cue.ParsePath(strLabel))
				if sv.Exists() && sv.IncompleteKind() == cue.StringKind {
					// Try getting concrete value; for disjunctions this may require default() resolution
					if s, err := sv.String(); err == nil && s != "" {
						step.Args[strLabel] = s
					} else {
						// Try via default value (for disjunction members)
						dv, _ := sv.Default()
						if ds, err := dv.String(); err == nil && ds != "" {
							step.Args[strLabel] = ds
						}
					}
				}
			}
		}

		// Double check args/params via explicit lookup if missed in loop
		for _, label := range []string{"args", "params"} {
			if _, ok := step.Args[label]; !ok && label != "params" {
				v := stepVal.LookupPath(cue.ParsePath(label))
				if v.Exists() {
					if v.Kind() == cue.ListKind {
						var p []string
						l, _ := v.List()
						for l.Next() {
							s, _ := l.Value().String()
							if s != "" {
								p = append(p, s)
							}
						}
						step.Args[label] = p
					} else if s, err := v.String(); err == nil && s != "" {
						step.Args[label] = []string{s}
					}
				}
			}
		}

		// Handle recursion for nested steps
		if v := stepVal.LookupPath(cue.ParsePath("then")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_then"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("else")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_else"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("do")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_do"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("cases")); v.Exists() && v.IncompleteKind() == cue.StructKind {
			casesMap := make(map[string][]FlowStep)
			cit, _ := v.Fields(cue.All())
			for cit.Next() {
				caseLabel := strings.Trim(cit.Selector().String(), "\"")
				caseVal := cit.Value()
				if !caseVal.Exists() || caseVal.Kind() != cue.ListKind {
					continue
				}
				if sub, err := n.parseFlowSteps(caseVal); err == nil {
					casesMap[caseLabel] = sub
				}
			}
			if len(casesMap) > 0 {
				step.Args["_cases"] = casesMap
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("default")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_default"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("ifNew")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_ifNew"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("ifExists")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_ifExists"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("catch")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_catch"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("fallback")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_fallback"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("onTimeout")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_onTimeout"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("onMissing")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_onMissing"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("onMismatch")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_onMismatch"] = sub
			}
		}
		// parallel.Run branches parsing
		if v := stepVal.LookupPath(cue.ParsePath("branches")); v.Exists() && v.IncompleteKind() == cue.StructKind {
			branchesMap := make(map[string][]FlowStep)
			bit, _ := v.Fields(cue.All())
			for bit.Next() {
				branchLabel := strings.Trim(bit.Selector().String(), "\"")
				branchVal := bit.Value()
				if !branchVal.Exists() || branchVal.Kind() != cue.ListKind {
					continue
				}
				if sub, err := n.parseFlowSteps(branchVal); err == nil {
					branchesMap[branchLabel] = sub
				}
			}
			if len(branchesMap) > 0 {
				step.Args["_branches"] = branchesMap
			}
		}

		steps = append(steps, step)
	}

	return steps, nil
}

// autoCompleteFlowSteps injects missing ID/CreatedAt fields before repo.Save for new entities
func (n *Normalizer) autoCompleteFlowSteps(steps []FlowStep) []FlowStep {
	assigned := make(map[string]bool)
	newEntities := make(map[string]bool)

	// First pass: identify new entities and assigned fields
	var scan func([]FlowStep)
	scan = func(steps []FlowStep) {
		for _, s := range steps {
			switch s.Action {
			case "mapping.Map":
				out := fmt.Sprint(s.Args["output"])
				if out == "" {
					out = fmt.Sprint(s.Args["to"])
				}
				if strings.HasPrefix(strings.ToLower(out), "new") {
					newEntities[out] = true
				}
			case "mapping.Assign":
				assigned[fmt.Sprint(s.Args["to"])] = true
			case "tx.Block", "flow.Block":
				if v, ok := s.Args["_do"].([]FlowStep); ok {
					scan(v)
				}
			case "repo.Upsert":
				if v, ok := s.Args["_ifNew"].([]FlowStep); ok {
					scan(v)
				}
				if v, ok := s.Args["_ifExists"].([]FlowStep); ok {
					scan(v)
				}
			case "flow.Switch":
				if v, ok := s.Args["_default"].([]FlowStep); ok {
					scan(v)
				}
				if cases, ok := s.Args["_cases"].(map[string][]FlowStep); ok {
					for _, branch := range cases {
						scan(branch)
					}
				}
			case "flow.If":
				if v, ok := s.Args["_then"].([]FlowStep); ok {
					scan(v)
				}
				if v, ok := s.Args["_else"].([]FlowStep); ok {
					scan(v)
				}
			case "flow.For":
				if v, ok := s.Args["_do"].([]FlowStep); ok {
					scan(v)
				}
			case "concurrency.Run", "bulkhead.Run", "circuit.Breaker", "trace.Span", "slo.Budget":
				if v, ok := s.Args["_do"].([]FlowStep); ok {
					scan(v)
				}
			case "batch.Run":
				if v, ok := s.Args["_do"].([]FlowStep); ok {
					scan(v)
				}
			case "flow.Try", "flow.Catch", "flow.Retry", "flow.Timeout":
				if v, ok := s.Args["_do"].([]FlowStep); ok {
					scan(v)
				}
				if v, ok := s.Args["_catch"].([]FlowStep); ok {
					scan(v)
				}
				if v, ok := s.Args["_onTimeout"].([]FlowStep); ok {
					scan(v)
				}
			case "flow.Fallback":
				if v, ok := s.Args["_do"].([]FlowStep); ok {
					scan(v)
				}
				if v, ok := s.Args["_fallback"].([]FlowStep); ok {
					scan(v)
				}
			case "flow.Resume":
				if v, ok := s.Args["_onMissing"].([]FlowStep); ok {
					scan(v)
				}
			case "flow.Replay":
				if v, ok := s.Args["_do"].([]FlowStep); ok {
					scan(v)
				}
				if v, ok := s.Args["_onMismatch"].([]FlowStep); ok {
					scan(v)
				}
			}
		}
	}
	scan(steps)

	// Second pass: inject missing fields before repo.Save
	var inject func([]FlowStep) []FlowStep
	inject = func(steps []FlowStep) []FlowStep {
		var result []FlowStep
		for _, s := range steps {
			if s.Action == "repo.Save" {
				input := fmt.Sprint(s.Args["input"])
				if newEntities[input] {
					// Inject ID if missing
					if !assigned[input+".ID"] {
						idVar := "_" + input + "AutoID"
						result = append(result, FlowStep{
							Action: "uuid.New",
							Args:   map[string]any{"output": idVar, "generated": "true"},
						})
						result = append(result, FlowStep{
							Action: "mapping.Assign",
							Args:   map[string]any{"to": input + ".ID", "value": idVar, "generated": "true"},
						})
						assigned[input+".ID"] = true
					}
					// Inject CreatedAt if missing
					if !assigned[input+".CreatedAt"] {
						createdAtVar := "_" + input + "AutoCreatedAt"
						result = append(result, FlowStep{
							Action: "time.Now",
							Args:   map[string]any{"output": createdAtVar, "format": "time.RFC3339", "generated": "true"},
						})
						result = append(result, FlowStep{
							Action: "mapping.Assign",
							Args:   map[string]any{"to": input + ".CreatedAt", "value": createdAtVar, "generated": "true"},
						})
						assigned[input+".CreatedAt"] = true
					}
				}
			}

			// Recurse into nested steps
			if v, ok := s.Args["_do"].([]FlowStep); ok {
				s.Args["_do"] = inject(v)
			}
			if v, ok := s.Args["_ifNew"].([]FlowStep); ok {
				s.Args["_ifNew"] = inject(v)
			}
			if v, ok := s.Args["_ifExists"].([]FlowStep); ok {
				s.Args["_ifExists"] = inject(v)
			}
			if v, ok := s.Args["_then"].([]FlowStep); ok {
				s.Args["_then"] = inject(v)
			}
			if v, ok := s.Args["_else"].([]FlowStep); ok {
				s.Args["_else"] = inject(v)
			}
			if v, ok := s.Args["_default"].([]FlowStep); ok {
				s.Args["_default"] = inject(v)
			}
			if v, ok := s.Args["_catch"].([]FlowStep); ok {
				s.Args["_catch"] = inject(v)
			}
			if v, ok := s.Args["_fallback"].([]FlowStep); ok {
				s.Args["_fallback"] = inject(v)
			}
			if v, ok := s.Args["_onTimeout"].([]FlowStep); ok {
				s.Args["_onTimeout"] = inject(v)
			}
			if v, ok := s.Args["_onMissing"].([]FlowStep); ok {
				s.Args["_onMissing"] = inject(v)
			}
			if v, ok := s.Args["_onMismatch"].([]FlowStep); ok {
				s.Args["_onMismatch"] = inject(v)
			}
			if cases, ok := s.Args["_cases"].(map[string][]FlowStep); ok {
				nextCases := make(map[string][]FlowStep, len(cases))
				for key, branch := range cases {
					nextCases[key] = inject(branch)
				}
				s.Args["_cases"] = nextCases
			}

			result = append(result, s)
		}
		return result
	}

	return inject(steps)
}

func flowStepIntArg(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, false
		}
		p, err := strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
		return p, true
	default:
		return 0, false
	}
}

// applyFlowPerformanceDefaults injects safe runtime defaults so generated code
// behaves predictably under load even when flow steps omit tuning knobs.
func (n *Normalizer) applyFlowPerformanceDefaults(steps []FlowStep) []FlowStep {
	var apply func([]FlowStep) []FlowStep
	apply = func(items []FlowStep) []FlowStep {
		out := make([]FlowStep, 0, len(items))
		for _, s := range items {
			if s.Args == nil {
				s.Args = map[string]any{}
			}

			switch s.Action {
			case "http.Call":
				if _, ok := s.Args["attempts"]; !ok {
					if retries, okRetries := flowStepIntArg(s.Args, "retries"); okRetries && retries >= 0 {
						s.Args["attempts"] = retries + 1
					} else {
						s.Args["attempts"] = 2
					}
				}
				if _, ok := s.Args["backoffMs"]; !ok {
					s.Args["backoffMs"] = 150
				}
				if _, hasTimeout := s.Args["timeout"]; !hasTimeout {
					if _, hasTimeoutMS := s.Args["timeoutMs"]; !hasTimeoutMS {
						s.Args["timeout"] = "5*time.Second"
					}
				}

			case "parallel.Run":
				if _, hasMaxConcurrency := s.Args["maxConcurrency"]; !hasMaxConcurrency {
					if _, hasMaxParallel := s.Args["maxParallel"]; !hasMaxParallel {
						s.Args["maxConcurrency"] = 8
					}
				}

			case "queue.Enqueue", "queue.Dequeue":
				if _, hasTimeout := s.Args["timeout"]; !hasTimeout {
					if _, hasTimeoutMS := s.Args["timeoutMs"]; !hasTimeoutMS {
						s.Args["timeout"] = "3*time.Second"
					}
				}
				if s.Action == "queue.Dequeue" {
					if _, hasAttempts := s.Args["attempts"]; !hasAttempts {
						if retries, okRetries := flowStepIntArg(s.Args, "retries"); okRetries && retries >= 0 {
							s.Args["attempts"] = retries + 1
						} else {
							s.Args["attempts"] = 2
						}
					}
					if _, hasBackoff := s.Args["backoffMs"]; !hasBackoff {
						s.Args["backoffMs"] = 150
					}
					if _, hasJitter := s.Args["jitterMs"]; !hasJitter {
						s.Args["jitterMs"] = 50
					}
				}
			}

			for _, key := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
				if nested, ok := s.Args[key].([]FlowStep); ok {
					s.Args[key] = apply(nested)
				}
			}
			if cases, ok := s.Args["_cases"].(map[string][]FlowStep); ok {
				next := make(map[string][]FlowStep, len(cases))
				for name, branch := range cases {
					next[name] = apply(branch)
				}
				s.Args["_cases"] = next
			}
			if branches, ok := s.Args["_branches"].(map[string][]FlowStep); ok {
				next := make(map[string][]FlowStep, len(branches))
				for name, branch := range branches {
					next[name] = apply(branch)
				}
				s.Args["_branches"] = next
			}

			out = append(out, s)
		}
		return out
	}
	return apply(steps)
}

func flowUsesObjectStorage(steps []FlowStep) bool {
	for _, step := range steps {
		switch step.Action {
		case "storage.Upload", "storage.Download", "storage.GetURL", "storage.Delete", "storage.List":
			return true
		}
		for _, key := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
			if nested, ok := step.Args[key].([]FlowStep); ok && flowUsesObjectStorage(nested) {
				return true
			}
		}
		if cases, ok := step.Args["_cases"].(map[string][]FlowStep); ok {
			for _, nested := range cases {
				if flowUsesObjectStorage(nested) {
					return true
				}
			}
		}
		if branches, ok := step.Args["_branches"].(map[string][]FlowStep); ok {
			for _, nested := range branches {
				if flowUsesObjectStorage(nested) {
					return true
				}
			}
		}
	}
	return false
}

// validateFlowSteps checks flow steps for common mistakes and returns warnings
type FlowWarning struct {
	Op           string
	Step         int
	Action       string
	Message      string
	Code         string
	Severity     string
	Hint         string
	File         string
	Line         int
	Column       int
	CUEPath      string
	SuggestedFix []Fix
}

func validateFlowSteps(opName string, svcName string, steps []FlowStep, entities []Entity, svcUses []string, policies map[string]PolicyDef, architectureMode string, allowCrossService map[string]map[string]struct{}) []FlowWarning {
	var warnings []FlowWarning
	seenWarnings := make(map[string]struct{})
	declaredVars := make(map[string]bool)
	assignedFields := make(map[string]bool)
	newEntities := make(map[string]string)

	entityOwners := make(map[string]string)
	entityContexts := make(map[string]string)
	aggregateOwnedByContext := make(map[string]map[string]struct{})
	isDTO := make(map[string]bool)
	isSharedArch := make(map[string]bool)
	for _, e := range entities {
		entityOwners[e.Name] = e.Owner
		ctx := strings.TrimSpace(strings.ToLower(e.BoundedContext))
		if ctx == "" {
			ctx = inferBoundedContext(e.Owner)
		}
		entityContexts[e.Name] = ctx
		if dto, ok := e.Metadata["dto"].(bool); ok && dto {
			isDTO[e.Name] = true
		}
		if shared, ok := e.Metadata["shared_arch"].(bool); ok && shared {
			isSharedArch[e.Name] = true
		}
	}
	for _, e := range entities {
		if !e.AggregateRoot {
			continue
		}
		rootCtx := strings.TrimSpace(strings.ToLower(e.BoundedContext))
		if rootCtx == "" {
			rootCtx = entityContexts[e.Name]
		}
		if rootCtx == "" {
			rootCtx = inferBoundedContext(e.Owner)
		}
		if rootCtx == "" {
			continue
		}
		ownedSet := aggregateOwnedByContext[rootCtx]
		if ownedSet == nil {
			ownedSet = make(map[string]struct{})
			aggregateOwnedByContext[rootCtx] = ownedSet
		}
		ownedSet[e.Name] = struct{}{}
		for _, owned := range e.Owns {
			owned = strings.TrimSpace(owned)
			if owned == "" {
				continue
			}
			ownedSet[owned] = struct{}{}
		}
	}
	serviceContext := inferBoundedContext(svcName)

	var currentStep FlowStep
	appendWarn := func(w FlowWarning) {
		key := fmt.Sprintf("%s|%s|%s|%d|%d|%d", w.Code, w.Action, w.File, w.Line, w.Column, w.Step)
		if _, ok := seenWarnings[key]; ok {
			return
		}
		seenWarnings[key] = struct{}{}
		warnings = append(warnings, w)
	}
	addWarn := func(step int, action, code, message, hint string, file string, line int, column int, fixes ...Fix) {
		appendWarn(FlowWarning{
			Op:           opName,
			Step:         step,
			Action:       action,
			Message:      message,
			Code:         code,
			Severity:     "error",
			Hint:         hint,
			File:         file,
			Line:         line,
			Column:       column,
			CUEPath:      currentStep.CUEPath,
			SuggestedFix: fixes,
		})
	}

	addWarnWithSeverity := func(step int, action, code, severity, message, hint string, file string, line int, column int, fixes ...Fix) {
		sev := strings.TrimSpace(strings.ToLower(severity))
		if sev == "" {
			sev = "error"
		}
		appendWarn(FlowWarning{
			Op:           opName,
			Step:         step,
			Action:       action,
			Message:      message,
			Code:         code,
			Severity:     sev,
			Hint:         hint,
			File:         file,
			Line:         line,
			Column:       column,
			CUEPath:      currentStep.CUEPath,
			SuggestedFix: fixes,
		})
	}

	archSeverity := "error"
	if strings.EqualFold(strings.TrimSpace(architectureMode), "relaxed") {
		archSeverity = "warn"
	}

	allowedDeps := map[string]struct{}{}
	allowedDeps[strings.ToLower(normalizeServiceName(svcName))] = struct{}{}
	for _, dep := range svcUses {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		allowedDeps[strings.ToLower(normalizeServiceName(dep))] = struct{}{}
	}

	isBoundaryViolation := func(entityName string) (bool, string, string) {
		owner, ok := entityOwners[entityName]
		if !ok {
			return false, "", ""
		}
		if strings.EqualFold(svcName, "admin") || strings.EqualFold(svcName, "audit") {
			return false, "", ""
		}
		if isSharedArch[entityName] {
			return false, "", ""
		}
		if serviceContext != "" {
			if ownedSet, ok := aggregateOwnedByContext[serviceContext]; ok {
				if _, allowed := ownedSet[entityName]; allowed {
					return false, "", ""
				}
			}
		}

		entityCtx := entityContexts[entityName]
		if entityCtx != "" && serviceContext != "" {
			if strings.EqualFold(entityCtx, serviceContext) {
				return false, "", ""
			}
			return true, fmt.Sprintf("bounded_context='%s'", entityCtx), entityCtx
		}

		ownerMatch := strings.EqualFold(owner, svcName) ||
			strings.EqualFold(owner+"s", svcName) ||
			strings.EqualFold(svcName+"s", owner)
		ownerPrefixMatch := strings.HasPrefix(strings.ToLower(owner), strings.ToLower(svcName)+"_")
		if owner != "" && !ownerMatch && !ownerPrefixMatch {
			return true, fmt.Sprintf("owned by '%s'", owner), owner
		}
		return false, "", ""
	}

	var validate func(steps []FlowStep, inTx bool, depth int)
	validate = func(steps []FlowStep, inTx bool, depth int) {
		for i := range steps {
			step := &steps[i]
			currentStep = *step
			stepNum := i + 1

			// fmt.Printf("DEBUG: checking action: '%s'\n", step.Action)
			switch step.Action {
			case "repo.Find", "repo.Get", "repo.GetForUpdate", "repo.Save", "repo.Delete", "repo.List", "repo.Query", "repo.Upsert",
				"db.Get", "db.List", "db.Query", "db.Insert", "db.Update", "db.Upsert", "db.Delete", "db.Lock", "db.SelectForUpdate":
				source, _ := step.Args["source"].(string)
				if source != "" {
					_, ok := entityOwners[source]

					if !ok {
						addWarn(stepNum, step.Action, "UNKNOWN_ENTITY",
							fmt.Sprintf("Entity '%s' is not defined in any domain CUE file", source),
							"Define the entity in cue/domain/ or check spelling", step.File, step.Line, step.Column)
					} else if isDTO[source] {
						addWarn(stepNum, step.Action, "DTO_AS_REPO",
							fmt.Sprintf("Entity '%s' is a DTO-only entity and cannot be accessed via repository", source),
							"Remove @dto(only=true) or use a real domain entity", step.File, step.Line, step.Column)
					}

					if violation, reason, targetService := isBoundaryViolation(source); ok && violation {
						filePath := filepath.ToSlash(strings.TrimSpace(step.File))
						if strings.Contains(filePath, "/cue/schema/") || strings.HasPrefix(filePath, "cue/schema/") {
							// Flow helper templates in cue/schema are generic and not tied to a real BC.
							// Skip architecture boundary enforcement for template source locations.
							continue
						}
						if !isCrossServiceAllowed(allowCrossService, svcName, source) {
							hintTarget := strings.TrimSpace(targetService)
							if hintTarget == "" {
								hintTarget = "target"
							}
							targetCtx := strings.TrimSpace(serviceContext)
							if targetCtx == "" {
								targetCtx = strings.TrimSpace(strings.ToLower(normalizeServiceName(svcName)))
							}
							addWarnWithSeverity(stepNum, step.Action, "ARCHITECTURE_VIOLATION", archSeverity,
								fmt.Sprintf("Service '%s' is not allowed to directly access entity '%s' (%s)", svcName, source, reason),
								fmt.Sprintf("Define a #ReadModel in '%s' bounded context with read_model.source_context='%s' and read_model.refreshOn=[...events], then read that model", targetCtx, hintTarget), step.File, step.Line, step.Column)
						}
					}
				}

				// Standard checks ...
				if strings.HasPrefix(step.Action, "repo.Find") || strings.HasPrefix(step.Action, "repo.Get") || step.Action == "repo.Upsert" ||
					step.Action == "db.Get" || step.Action == "db.Lock" || step.Action == "db.SelectForUpdate" || step.Action == "db.Upsert" {
					output, _ := step.Args["output"].(string)
					if output != "" {
						declaredVars[output] = true
					}
				}
				if step.Action == "repo.List" || step.Action == "repo.Query" || step.Action == "db.List" || step.Action == "db.Query" {
					output, _ := step.Args["output"].(string)
					if output != "" {
						declaredVars[output] = true
					}
				}
				if step.Action == "repo.Query" || step.Action == "db.Query" {
					if step.Args["method"] == nil || step.Args["method"] == "" {
						addWarn(stepNum, step.Action, "MISSING_METHOD", step.Action+" missing 'method'", fmt.Sprintf("{action: \"%s\", source: \"Entity\", method: \"ListBy...\", input: \"...\", output: \"items\"}", step.Action), step.File, step.Line, step.Column)
					}
					if input, _ := step.Args["input"].(string); strings.TrimSpace(input) != "" {
						if errStr := validateSafeCallArgExpr(input); errStr != "" {
							addWarn(stepNum, step.Action, "UNSAFE_QUERY_ARG_EXPR", step.Action+" input: "+errStr, "Use only refs/literals (or safe struct literals without calls) in query inputs.", step.File, step.Line, step.Column)
						}
					}
					// Normalize args to []string (CUE lists arrive as []interface{})
					if args, ok := step.Args["args"]; ok {
						switch v := args.(type) {
						case string:
							step.Args["args"] = []string{v}
						case []any:
							var ss []string
							for _, x := range v {
								ss = append(ss, fmt.Sprint(x))
							}
							step.Args["args"] = ss
						}
					}
					if args, ok := step.Args["args"].([]string); ok {
						for idx, arg := range args {
							if errStr := validateSafeCallArgExpr(arg); errStr != "" {
								addWarn(stepNum, step.Action, "UNSAFE_QUERY_ARG_EXPR", fmt.Sprintf("%s args[%d]: %s", step.Action, idx, errStr), "Use only refs/literals (or safe struct literals without calls) in query args.", step.File, step.Line, step.Column)
							}
						}
					}
				}
				if (step.Action == "repo.GetForUpdate" || step.Action == "db.Lock" || step.Action == "db.SelectForUpdate") && !inTx {
					addWarn(stepNum, step.Action, "TX_REQUIRED", step.Action+" outside tx.Block", "{action: \"tx.Block\", do: [ ... ]}", step.File, step.Line, step.Column)
				}
				if step.Action == "repo.Upsert" {
					if step.Args["source"] == nil || step.Args["source"] == "" {
						addWarn(stepNum, step.Action, "MISSING_SOURCE", "repo.Upsert missing 'source'", "{action: \"repo.Upsert\", source: \"Entity\", find: \"FindBy...\", input: \"...\", output: \"item\"}", step.File, step.Line, step.Column)
					}
					if step.Args["find"] == nil || step.Args["find"] == "" {
						addWarn(stepNum, step.Action, "MISSING_FIND", "repo.Upsert missing 'find'", "{action: \"repo.Upsert\", source: \"Entity\", find: \"FindBy...\", input: \"...\", output: \"item\"}", step.File, step.Line, step.Column)
					}
					if step.Args["input"] == nil || step.Args["input"] == "" {
						addWarn(stepNum, step.Action, "MISSING_INPUT", "repo.Upsert missing 'input'", "{action: \"repo.Upsert\", source: \"Entity\", find: \"FindBy...\", input: \"...\", output: \"item\"}", step.File, step.Line, step.Column)
					}
					if step.Args["output"] == nil || step.Args["output"] == "" {
						addWarn(stepNum, step.Action, "MISSING_OUTPUT", "repo.Upsert missing 'output'", "{action: \"repo.Upsert\", source: \"Entity\", find: \"FindBy...\", input: \"...\", output: \"item\"}", step.File, step.Line, step.Column)
					}
					_, hasIfNew := step.Args["_ifNew"].([]FlowStep)
					_, hasIfExists := step.Args["_ifExists"].([]FlowStep)
					if !hasIfNew && !hasIfExists {
						addWarn(stepNum, step.Action, "MISSING_BRANCHES", "repo.Upsert requires at least one branch: ifNew or ifExists", "{action: \"repo.Upsert\", ..., ifNew: [ ... ]}", step.File, step.Line, step.Column)
					}
					if subSteps, ok := step.Args["_ifNew"].([]FlowStep); ok {
						validate(subSteps, inTx, depth+1)
					}
					if subSteps, ok := step.Args["_ifExists"].([]FlowStep); ok {
						validate(subSteps, inTx, depth+1)
					}
				}

			case "mapping.Map":
				output, _ := step.Args["output"].(string)
				if output == "" {
					output, _ = step.Args["to"].(string)
				}
				entity, _ := step.Args["entity"].(string)
				if output != "" && strings.HasPrefix(strings.ToLower(output), "new") && entity == "" {
					addWarn(stepNum, step.Action, "MISSING_ENTITY", fmt.Sprintf("mapping.Map '%s' missing 'entity'", output), "{action: \"mapping.Map\", output: \""+output+"\", entity: \"Entity\"}", step.File, step.Line, step.Column)
				}
				if output != "" && entity != "" {
					declaredVars[output] = true
					newEntities[output] = entity
				}

			case "mapping.Assign":
				to, _ := step.Args["to"].(string)
				value, _ := step.Args["value"].(string)
				if to == "" {
					addWarn(stepNum, step.Action, "MISSING_TO", "mapping.Assign missing 'to'", "{action: \"mapping.Assign\", to: \"x.Field\", value: \"...\"}", step.File, step.Line, step.Column)
				}
				if value == "" {
					addWarn(stepNum, step.Action, "MISSING_VALUE", "mapping.Assign missing 'value'", "{action: \"mapping.Assign\", to: \"x.Field\", value: \"...\"}", step.File, step.Line, step.Column)
				}
				assignedFields[to] = true

				// Keep normalizer-level checks for backward compatibility;
				// flowsem now also validates these expressions.
				if errStr := validateValueExpr(value); errStr != "" {
					addWarn(stepNum, step.Action, "GO_SYNTAX_ERROR", errStr, "Check your Go expression syntax inside the CUE string.", step.File, step.Line, step.Column)
				}
				if errStr := validateMappingAssignSafeValue(value); errStr != "" {
					addWarn(stepNum, step.Action, "MAPPING_ASSIGN_UNSAFE_EXPR", errStr, "Use safe references/literals in mapping.Assign, and move function calls to typed actions (uuid.New, time.Now, rand.Token, ...).", step.File, step.Line, step.Column)
				}

				// Check for unquoted status strings
				statusValues := map[string]bool{"draft": true, "active": true, "pending": true, "published": true, "closed": true, "approved": true, "rejected": true, "cancelled": true}
				if value != "" && !strings.Contains(value, "\"") && !strings.Contains(value, ".") && !strings.Contains(value, "(") {
					if statusValues[strings.ToLower(value)] {
						addWarn(stepNum, step.Action, "NEEDS_QUOTES", fmt.Sprintf("mapping.Assign '%s' needs quotes: \"\\\"%s\\\"\"", value, value), "{action: \"mapping.Assign\", to: \"x.Status\", value: \"\\\""+value+"\\\"\"}", step.File, step.Line, step.Column, Fix{
							Kind: "replace",
							Text: "\"" + value + "\"",
						})
					}
				}

			case "event.Publish":
				name, _ := step.Args["name"].(string)
				payload, _ := step.Args["payload"].(string)
				payloadMap := flowStringMap(step.Args["payloadMap"])
				if payload != "" {
					addWarn(stepNum, step.Action, "EVENT_PUBLISH_RAW_PAYLOAD_FORBIDDEN", "event.Publish raw 'payload' is not allowed; use payloadMap", "{action: \"event.Publish\", name: \""+name+"\", payloadMap: { TenderID: \"req.TenderID\" }}", step.File, step.Line, step.Column)
				}
				for field, expr := range payloadMap {
					field = strings.TrimSpace(field)
					if field == "" {
						addWarn(stepNum, step.Action, "INVALID_PAYLOAD_FIELD", "event.Publish payloadMap contains empty field name", "{action: \"event.Publish\", payloadMap: { Field: \"req.Value\" }}", step.File, step.Line, step.Column)
						continue
					}
					if errStr := validateSafeCallArgExpr(expr); errStr != "" {
						addWarn(stepNum, step.Action, "UNSAFE_EVENT_PAYLOAD_EXPR", fmt.Sprintf("event.Publish payloadMap[%q]: %s", field, errStr), "Use only refs/literals (or safe struct literals without calls) in payloadMap values.", step.File, step.Line, step.Column)
					}
				}

			case "event.Broadcast":
				// validated by flow semantics engine

			case "event.Wait":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "event.Subscribe":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "event.Match":
				// validated by flow semantics engine

			case "notification.Dispatch":
				// validated by flow semantics engine

			case "notify.Send":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}
				// validated by flow semantics engine

			case "logic.Check":
				cond, _ := step.Args["condition"].(string)
				if cond != "" {
					if errStr := validateSafeConditionExpr(cond); errStr != "" {
						addWarn(stepNum, step.Action, "UNSAFE_CONDITION_EXPR", "logic.Check condition: "+errStr, "Use safe boolean refs/comparisons only; avoid calls and Go-specific constructs.", step.File, step.Line, step.Column)
					}
				}

			case "logic.Call":
				if step.Args["func"] == nil || step.Args["func"] == "" {
					addWarn(stepNum, step.Action, "MISSING_FUNC", "logic.Call missing 'func'", "{action: \"logic.Call\", func: \"DoThing\", args: [\"a\", \"b\"]}", step.File, step.Line, step.Column)
				}
				// Normalize args to []string for templates
				if args, ok := step.Args["args"]; ok {
					switch v := args.(type) {
					case string:
						step.Args["args"] = []string{v}
					case []any:
						var ss []string
						for _, x := range v {
							ss = append(ss, fmt.Sprint(x))
						}
						step.Args["args"] = ss
					}
				} else {
					step.Args["args"] = []string{}
				}
				if args, ok := step.Args["args"].([]string); ok {
					for idx, arg := range args {
						if errStr := validateSafeCallArgExpr(arg); errStr != "" {
							addWarn(stepNum, step.Action, "UNSAFE_CALL_ARG_EXPR", fmt.Sprintf("logic.Call args[%d]: %s", idx, errStr), "Use only refs/literals (or safe struct literals without calls) in call args.", step.File, step.Line, step.Column)
						}
					}
				}

			case "service.Call":
				serviceTarget, _ := step.Args["service"].(string)
				methodTarget, _ := step.Args["method"].(string)
				if strings.TrimSpace(serviceTarget) == "" {
					addWarn(stepNum, step.Action, "MISSING_SERVICE", "service.Call missing 'service'", "{action: \"service.Call\", service: \"Tender\", method: \"GetTender\", args: [\"ctx\", \"req\"]}", step.File, step.Line, step.Column)
				}
				if strings.TrimSpace(methodTarget) == "" {
					addWarn(stepNum, step.Action, "MISSING_METHOD", "service.Call missing 'method'", "{action: \"service.Call\", service: \"Tender\", method: \"GetTender\", args: [\"ctx\", \"req\"]}", step.File, step.Line, step.Column)
				}
				dep := strings.ToLower(normalizeServiceName(serviceTarget))
				if dep != "" {
					if _, ok := allowedDeps[dep]; !ok {
						addWarn(stepNum, step.Action, "MISSING_SERVICE_DEP", fmt.Sprintf("service.Call targets '%s' but service '%s' does not declare it in uses", serviceTarget, svcName), "Add service name to operation uses: uses: [\""+serviceTarget+"\"]", step.File, step.Line, step.Column)
					}
				}
				if args, ok := step.Args["args"]; ok {
					switch v := args.(type) {
					case string:
						step.Args["args"] = []string{v}
					case []any:
						var ss []string
						for _, x := range v {
							ss = append(ss, fmt.Sprint(x))
						}
						step.Args["args"] = ss
					}
				} else {
					step.Args["args"] = []string{}
				}
				if args, ok := step.Args["args"].([]string); ok {
					for idx, arg := range args {
						if errStr := validateSafeCallArgExpr(arg); errStr != "" {
							addWarn(stepNum, step.Action, "UNSAFE_CALL_ARG_EXPR", fmt.Sprintf("service.Call args[%d]: %s", idx, errStr), "Use only refs/literals (or safe struct literals without calls) in call args.", step.File, step.Line, step.Column)
						}
					}
				}
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "flow.Call":
				opRaw, _ := step.Args["op"].(string)
				opRaw = strings.TrimSpace(opRaw)
				if opRaw == "" {
					addWarn(stepNum, step.Action, "MISSING_OP", "flow.Call missing 'op'", "{action: \"flow.Call\", op: \"ValidateTenderForBid\", args: {tenderID: \"req.TenderID\"}}", step.File, step.Line, step.Column)
					break
				}

				targetService := ""
				targetMethod := opRaw
				if strings.Contains(opRaw, ".") {
					parts := strings.SplitN(opRaw, ".", 2)
					targetService = strings.TrimSpace(parts[0])
					targetMethod = strings.TrimSpace(parts[1])
				}
				if targetMethod == "" {
					addWarn(stepNum, step.Action, "INVALID_OP", "flow.Call op must be MethodName or ServiceName.MethodName", "{action: \"flow.Call\", op: \"Tender.ValidateTenderForBid\"}", step.File, step.Line, step.Column)
				}
				if targetService != "" {
					dep := strings.ToLower(normalizeServiceName(targetService))
					if _, ok := allowedDeps[dep]; !ok {
						addWarn(stepNum, step.Action, "MISSING_SERVICE_DEP", fmt.Sprintf("flow.Call targets '%s' but service '%s' does not declare it in uses", targetService, svcName), "Add service name to operation uses: uses: [\""+targetService+"\"]", step.File, step.Line, step.Column)
					}
				}

				if argsRaw, ok := step.Args["args"]; ok {
					switch v := argsRaw.(type) {
					case map[string]string:
						for key, expr := range v {
							if errStr := validateSafeCallArgExpr(expr); errStr != "" {
								addWarn(stepNum, step.Action, "UNSAFE_CALL_ARG_EXPR", fmt.Sprintf("flow.Call args.%s: %s", key, errStr), "Use only refs/literals (or safe struct literals without calls) in flow.Call args.", step.File, step.Line, step.Column)
							}
						}
					case map[string]any:
						for key, raw := range v {
							expr := fmt.Sprint(raw)
							if errStr := validateSafeCallArgExpr(expr); errStr != "" {
								addWarn(stepNum, step.Action, "UNSAFE_CALL_ARG_EXPR", fmt.Sprintf("flow.Call args.%s: %s", key, errStr), "Use only refs/literals (or safe struct literals without calls) in flow.Call args.", step.File, step.Line, step.Column)
							}
						}
					default:
						addWarn(stepNum, step.Action, "INVALID_ARGS_TYPE", "flow.Call args must be map[string]string", "{action: \"flow.Call\", op: \"ValidateTenderForBid\", args: {tenderID: \"req.TenderID\"}}", step.File, step.Line, step.Column)
					}
				}
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "list.Append":
				// validated by flow semantics engine

			case "fsm.Transition":
				// validated by flow semantics engine

			case "tx.Block", "flow.Block":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, step.Action == "tx.Block", depth+1)
				}

			case "flow.If":
				if subSteps, ok := step.Args["_then"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}
				if subSteps, ok := step.Args["_else"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.Switch":
				cases, ok := step.Args["_cases"].(map[string][]FlowStep)
				if ok {
					for _, subSteps := range cases {
						validate(subSteps, inTx, depth+1)
					}
				}
				if subSteps, ok := step.Args["_default"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.For":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "batch.Run":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.Try", "flow.Retry", "flow.Timeout":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}
				if subSteps, ok := step.Args["_catch"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}
				if subSteps, ok := step.Args["_onTimeout"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.Catch":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.Fallback":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}
				if subSteps, ok := step.Args["_fallback"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.Resume":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}
				if subSteps, ok := step.Args["_onMissing"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.Replay":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}
				if subSteps, ok := step.Args["_onMismatch"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.Saga":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.Compensate":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.Rollback":
				// validated by flow semantics engine

			case "flow.Tag":
				// validated by flow semantics engine

			case "flow.SuggestNext", "flow.ExplainError":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "flow.RecordEvent", "flow.History.Get":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "approval.Request":
				if approvalID, _ := step.Args["approvalId"].(string); approvalID != "" {
					declaredVars[approvalID] = true
				}
				if status, _ := step.Args["status"].(string); status != "" {
					declaredVars[status] = true
				}

			case "approval.Wait":
				for _, key := range []string{"decision", "status", "decidedBy", "decidedAt", "reason"} {
					if out, _ := step.Args[key].(string); out != "" {
						declaredVars[out] = true
					}
				}
				if subSteps, ok := step.Args["_onTimeout"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "approval.Decide":
				if status, _ := step.Args["status"].(string); status != "" {
					declaredVars[status] = true
				}

			case "policy.Check":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}
				policyName, _ := step.Args["policy"].(string)
				policyName = strings.TrimSpace(policyName)
				if u, err := strconv.Unquote(policyName); err == nil {
					policyName = strings.TrimSpace(u)
				}
				if policyName == "" {
					addWarn(stepNum, step.Action, "MISSING_POLICY", "policy.Check missing 'policy'", "{action: \"policy.Check\", policy: \"CompanyAdminOnly\", user: \"currentUser\", companyID: \"req.CompanyID\"}", step.File, step.Line, step.Column)
					break
				}
				if len(policies) == 0 {
					addWarn(stepNum, step.Action, "POLICY_REGISTRY_EMPTY", "policy.Check used but #Policies registry is empty (expected cue/policy/*.cue)", "Define #Policies map with typed policies and reload build", step.File, step.Line, step.Column)
					break
				}
				p, ok := policies[policyName]
				if !ok {
					addWarn(stepNum, step.Action, "UNKNOWN_POLICY", fmt.Sprintf("policy '%s' is not defined in #Policies", policyName), "Allowed: "+strings.Join(sortedPolicyNames(policies), ", "), step.File, step.Line, step.Column)
					break
				}
				if p.SameCompany {
					companyExpr, _ := step.Args["companyID"].(string)
					if strings.TrimSpace(companyExpr) == "" {
						addWarn(stepNum, step.Action, "MISSING_COMPANY_ID", fmt.Sprintf("policy '%s' requires companyID (sameCompany=true)", policyName), "{action: \"policy.Check\", policy: \""+policyName+"\", user: \"currentUser\", companyID: \"req.CompanyID\"}", step.File, step.Line, step.Column)
					}
				}

			case "policy.Evaluate", "policy.Require", "policy.Decide":
				for _, key := range []string{"decision", "reason", "effects", "output"} {
					if out, _ := step.Args[key].(string); out != "" {
						declaredVars[out] = true
					}
				}

			case "audit.Log":
				// validated by flow semantics engine

			case "auth.RequireRole":
				// Register output variable
				if authOutput, _ := step.Args["output"].(string); authOutput != "" {
					declaredVars[authOutput] = true
				} else {
					declaredVars["currentUser"] = true
				}

			case "auth.CheckRole":
				// validated by flow semantics engine

			case "rbac.CheckPermission":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "entity.PatchNonZero":
				// validated by flow semantics engine

			case "entity.PatchValidated":
				fields, ok := step.Args["fields"].(map[string]map[string]string)
				if !ok || len(fields) == 0 {
					break
				}
				for _, rules := range fields {
					if uniqueMethod := strings.TrimSpace(rules["unique"]); uniqueMethod != "" {
						if step.Args["source"] == nil || step.Args["source"] == "" {
							addWarn(stepNum, step.Action, "MISSING_SOURCE", "entity.PatchValidated with unique checks requires explicit 'source' repository entity", "{action: \"entity.PatchValidated\", source: \"Company\", ...}", step.File, step.Line, step.Column)
						}
					}
				}

			case "field.CopyNonEmpty":
				// validated by flow semantics engine

			case "list.Paginate":
				// validated by flow semantics engine

			case "str.Normalize":
				// validated by flow semantics engine

			case "enum.Validate":
				// validated by flow semantics engine

			case "list.Sort":
				// validated by flow semantics engine

			case "list.Filter":
				// validated by flow semantics engine

			case "list.Len", "list.New", "map.New":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "list.Map", "list.Reduce", "list.GroupBy", "list.Distinct", "list.Chunk":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "list.Enrich":
				if step.Args["items"] == nil || step.Args["items"] == "" {
					addWarn(stepNum, step.Action, "MISSING_ITEMS", "list.Enrich missing 'items'", "{action: \"list.Enrich\", items: \"items\", lookupSource: \"Company\", lookupInput: \"item.CompanyID\", set: \"Name=Name\"}", step.File, step.Line, step.Column)
				}
				if step.Args["lookupSource"] == nil || step.Args["lookupSource"] == "" {
					addWarn(stepNum, step.Action, "MISSING_LOOKUP_SOURCE", "list.Enrich missing 'lookupSource'", "{action: \"list.Enrich\", items: \"items\", lookupSource: \"Company\", lookupInput: \"item.CompanyID\", set: \"Name=Name\"}", step.File, step.Line, step.Column)
				}
				lookupSource, _ := step.Args["lookupSource"].(string)
				if lookupSource != "" {
					_, ok := entityOwners[lookupSource]
					if !ok {
						addWarn(stepNum, step.Action, "UNKNOWN_ENTITY", fmt.Sprintf("Entity '%s' is not defined in any domain CUE file", lookupSource), "Define the entity in cue/domain/ or check spelling", step.File, step.Line, step.Column)
					} else if isDTO[lookupSource] {
						addWarn(stepNum, step.Action, "DTO_AS_REPO", fmt.Sprintf("Entity '%s' is a DTO-only entity and cannot be accessed via repository", lookupSource), "Remove @dto(only=true) or use a real domain entity", step.File, step.Line, step.Column)
					}
					if violation, reason, targetService := isBoundaryViolation(lookupSource); ok && violation {
						if !isCrossServiceAllowed(allowCrossService, svcName, lookupSource) {
							hintTarget := strings.TrimSpace(targetService)
							if hintTarget == "" {
								hintTarget = "target"
							}
							addWarnWithSeverity(stepNum, step.Action, "ARCHITECTURE_VIOLATION", archSeverity, fmt.Sprintf("Service '%s' is not allowed to directly access entity '%s' (%s)", svcName, lookupSource, reason), fmt.Sprintf("Use events or call %sService", strings.Title(hintTarget)), step.File, step.Line, step.Column)
						}
					}
				}
				// required args / set format validated by flow semantics engine

			case "time.Parse":
				// validated by flow semantics engine

			case "time.CheckExpiry":
				// validated by flow semantics engine

			case "map.Build", "time.Now", "time.Format":
				// validated by flow semantics engine

			case "exec.Run":
				if step.Args["cmd"] == nil || step.Args["cmd"] == "" {
					addWarn(stepNum, step.Action, "MISSING_CMD", "exec.Run missing 'cmd'", "{action: \"exec.Run\", cmd: \"/usr/bin/ang\", args: [\"build\"], output: \"result\"}", step.File, step.Line, step.Column)
				}
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "fs.TempDir":
				if output, _ := step.Args["output"].(string); output == "" {
					addWarn(stepNum, step.Action, "MISSING_OUTPUT", "fs.TempDir missing 'output'", "{action: \"fs.TempDir\", output: \"workDir\", pattern: \"sendbox-*\"}", step.File, step.Line, step.Column)
				} else {
					declaredVars[output] = true
				}

			case "fs.WriteFile":
				if step.Args["path"] == nil || step.Args["path"] == "" {
					addWarn(stepNum, step.Action, "MISSING_PATH", "fs.WriteFile missing 'path'", "{action: \"fs.WriteFile\", path: \"filePath\", data: \"req.Content\"}", step.File, step.Line, step.Column)
				}
				if step.Args["data"] == nil || step.Args["data"] == "" {
					addWarn(stepNum, step.Action, "MISSING_DATA", "fs.WriteFile missing 'data'", "{action: \"fs.WriteFile\", path: \"filePath\", data: \"req.Content\"}", step.File, step.Line, step.Column)
				}

			case "fs.ReadFile":
				if step.Args["path"] == nil || step.Args["path"] == "" {
					addWarn(stepNum, step.Action, "MISSING_PATH", "fs.ReadFile missing 'path'", "{action: \"fs.ReadFile\", path: \"filePath\", output: \"contents\"}", step.File, step.Line, step.Column)
				}
				if output, _ := step.Args["output"].(string); output == "" {
					addWarn(stepNum, step.Action, "MISSING_OUTPUT", "fs.ReadFile missing 'output'", "{action: \"fs.ReadFile\", path: \"filePath\", output: \"contents\"}", step.File, step.Line, step.Column)
				} else {
					declaredVars[output] = true
				}

			case "fs.Remove":
				if step.Args["path"] == nil || step.Args["path"] == "" {
					addWarn(stepNum, step.Action, "MISSING_PATH", "fs.Remove missing 'path'", "{action: \"fs.Remove\", path: \"workDir\"}", step.File, step.Line, step.Column)
				}

			case "cache.Get", "cache.Set", "cache.Del":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "mail.Send":
				// validated by flow semantics engine

			case "storage.Upload", "storage.Download", "storage.GetURL", "storage.Delete", "storage.List":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "queue.Enqueue", "queue.Ack", "queue.Nack", "dlq.Publish", "event.Outbox", "webhook.Ack":
				// validated by flow semantics engine

			case "queue.Dequeue":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}
				if ackToken, _ := step.Args["ackToken"].(string); ackToken != "" {
					declaredVars[ackToken] = true
				}

			case "webhook.Send":
				// validated by flow semantics engine

			case "webhook.VerifySignature":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "http.Call", "http.Request", "http.RetryPolicy":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}
				if statusVar, _ := step.Args["statusVar"].(string); statusVar != "" {
					declaredVars[statusVar] = true
				}

			case "http.Paginate":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "rand.Code", "rand.Token",
				"uuid.New", "ulid.New",
				"regex.Match", "regex.Replace",
				"base64.Encode", "base64.Decode",
				"url.Parse", "url.Build",
				"query.Encode", "query.Decode",
				"hash.Sum", "hash.HMAC",
				"str.Format", "str.Concat",
				"cast.ToString",
				"json.Parse", "json.Marshal",
				"math.Op",
				"num.Add", "num.Sub", "num.Mul", "num.Div",
				"jsonpath.Get", "jsonpath.Set",
				"jwt.Sign", "jwt.Verify",
				"oauth2.Token", "oauth2.Refresh",
				"crypto.Encrypt", "crypto.Decrypt":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "parallel.Run":
				if branches, ok := step.Args["_branches"].(map[string][]FlowStep); ok {
					for _, branchSteps := range branches {
						validate(branchSteps, inTx, depth+1)
					}
				}

			case "state.Get":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "state.Set", "state.Delete":
				// validated by flow semantics engine

			case "idem.DeriveKey", "idempotency.DeriveKey":
				if output, _ := step.Args["output"].(string); output != "" {
					declaredVars[output] = true
				}

			case "idem.Check", "idem.SaveResult", "idempotency.Check", "idempotency.SaveResult":
				// validated by flow semantics engine

			case "dedupe.Once":
				if doSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(doSteps, inTx, depth+1)
				}

			case "ratelimit.Check", "concurrency.Limit",
				"circuit.Check", "circuit.RecordSuccess", "circuit.RecordFailure",
				"bulkhead.Acquire", "ratelimit.Limit", "log.Emit", "metric.Emit":
				// validated by flow semantics engine

			case "concurrency.Run", "bulkhead.Run", "circuit.Breaker", "trace.Span", "slo.Budget":
				if doSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(doSteps, inTx, depth+1)
				}
				// validated by flow semantics engine

			case "secret.Get", "config.Get":
				if step.Args["key"] == nil || step.Args["key"] == "" {
					addWarn(stepNum, step.Action, "MISSING_KEY", fmt.Sprintf("%s missing 'key'", step.Action), fmt.Sprintf("{action: \"%s\", key: \"KEY_NAME\", output: \"val\"}", step.Action), step.File, step.Line, step.Column)
				}
				if output, _ := step.Args["output"].(string); output == "" {
					addWarn(stepNum, step.Action, "MISSING_OUTPUT", fmt.Sprintf("%s missing 'output'", step.Action), fmt.Sprintf("{action: \"%s\", key: \"KEY_NAME\", output: \"val\"}", step.Action), step.File, step.Line, step.Column)
				} else {
					declaredVars[output] = true
				}

			default:
				if step.Action != "" && !strings.HasPrefix(step.Action, "repo.") && !strings.HasPrefix(step.Action, "mapping.") &&
					!strings.HasPrefix(step.Action, "logic.") && !strings.HasPrefix(step.Action, "event.") &&
					!strings.HasPrefix(step.Action, "fsm.") && !strings.HasPrefix(step.Action, "flow.") &&
					!strings.HasPrefix(step.Action, "tx.") && !strings.HasPrefix(step.Action, "list.") &&
					!strings.HasPrefix(step.Action, "notification.") &&
					!strings.HasPrefix(step.Action, "notify.") &&
					!strings.HasPrefix(step.Action, "approval.") &&
					!strings.HasPrefix(step.Action, "policy.") &&
					!strings.HasPrefix(step.Action, "audit.") && !strings.HasPrefix(step.Action, "auth.") &&
					!strings.HasPrefix(step.Action, "entity.") && !strings.HasPrefix(step.Action, "field.") &&
					!strings.HasPrefix(step.Action, "str.") && !strings.HasPrefix(step.Action, "enum.") &&
					!strings.HasPrefix(step.Action, "time.") && !strings.HasPrefix(step.Action, "map.") &&
					!strings.HasPrefix(step.Action, "exec.") && !strings.HasPrefix(step.Action, "fs.") &&
					!strings.HasPrefix(step.Action, "cache.") && !strings.HasPrefix(step.Action, "mail.") &&
					!strings.HasPrefix(step.Action, "storage.") && !strings.HasPrefix(step.Action, "http.") &&
					!strings.HasPrefix(step.Action, "webhook.") && !strings.HasPrefix(step.Action, "queue.") &&
					!strings.HasPrefix(step.Action, "rand.") && !strings.HasPrefix(step.Action, "json.") &&
					!strings.HasPrefix(step.Action, "regex.") && !strings.HasPrefix(step.Action, "base64.") &&
					!strings.HasPrefix(step.Action, "url.") && !strings.HasPrefix(step.Action, "query.") &&
					!strings.HasPrefix(step.Action, "hash.") && !strings.HasPrefix(step.Action, "uuid.") &&
					!strings.HasPrefix(step.Action, "ulid.") && !strings.HasPrefix(step.Action, "math.") &&
					!strings.HasPrefix(step.Action, "jsonpath.") &&
					!strings.HasPrefix(step.Action, "jwt.") &&
					!strings.HasPrefix(step.Action, "oauth2.") &&
					!strings.HasPrefix(step.Action, "crypto.") &&
					!strings.HasPrefix(step.Action, "rbac.") &&
					!strings.HasPrefix(step.Action, "batch.") &&
					!strings.HasPrefix(step.Action, "parallel.") &&
					!strings.HasPrefix(step.Action, "archive.") &&
					!strings.HasPrefix(step.Action, "session.") &&
					!strings.HasPrefix(step.Action, "idem.") &&
					!strings.HasPrefix(step.Action, "idempotency.") &&
					!strings.HasPrefix(step.Action, "dedupe.") &&
					!strings.HasPrefix(step.Action, "ratelimit.") &&
					!strings.HasPrefix(step.Action, "concurrency.") &&
					!strings.HasPrefix(step.Action, "circuit.") &&
					!strings.HasPrefix(step.Action, "bulkhead.") &&
					!strings.HasPrefix(step.Action, "log.") &&
					!strings.HasPrefix(step.Action, "metric.") &&
					!strings.HasPrefix(step.Action, "trace.") &&
					!strings.HasPrefix(step.Action, "slo.") &&
					!strings.HasPrefix(step.Action, "state.") &&
					!strings.HasPrefix(step.Action, "dlq.") &&
					!strings.HasPrefix(step.Action, "db.") {
					addWarn(stepNum, step.Action, "UNKNOWN_ACTION", fmt.Sprintf("unknown action '%s'", step.Action), "{action: \"repo.Find\" | \"mapping.Assign\" | \"flow.If\" ...}", step.File, step.Line, step.Column)
				}
			}
		}
	}

	validate(steps, false, 0)
	return warnings
}
