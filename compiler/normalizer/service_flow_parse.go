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
