package normalizer

import (
	"fmt"
	"strconv"
	"strings"
)

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
