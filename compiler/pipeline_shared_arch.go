package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func emitSharedArchDiagnostics(entities []normalizer.Entity, services []normalizer.Service, opts PipelineOptions) {
	usageByEntity := collectEntityContextUsage(services)
	for _, ent := range entities {
		if !isSharedArchEntity(ent) {
			continue
		}

		file, line := parseSourcePos(ent.Source)
		reason := strings.TrimSpace(toMetadataString(ent.Metadata["shared_arch_reason"]))
		ticket := strings.TrimSpace(toMetadataString(ent.Metadata["shared_arch_ticket"]))
		usageContexts := sortedSetKeys(usageByEntity[ent.Name])

		usageSummary := "used by contexts: none"
		if len(usageContexts) > 0 {
			usageSummary = "used by contexts: " + strings.Join(usageContexts, ", ")
		}
		reasonSummary := "reason: <missing>"
		if reason != "" {
			reasonSummary = "reason: " + reason
		}
		if ticket != "" {
			reasonSummary += " (ticket: " + ticket + ")"
		}

		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "architecture",
			Code:     "SHARED_ARCH_AUDIT",
			Severity: "warn",
			Message:  fmt.Sprintf("Entity '%s' is marked shared_arch (%s; %s)", ent.Name, reasonSummary, usageSummary),
			File:     file,
			Line:     line,
			Hint:     "Keep shared_arch temporary. Prefer #ReadModel/event-driven integration for cross-context reads.",
		}, opts)

		if reason == "" {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "architecture",
				Code:     "SHARED_ARCH_REASON_REQUIRED",
				Severity: "error",
				Message:  fmt.Sprintf("Entity '%s' uses shared_arch but has no reason", ent.Name),
				File:     file,
				Line:     line,
				Hint:     `Add explicit rationale: @shared_arch(reason="...") or shared_arch_reason: "..."`,
			}, opts)
		}

		if len(usageContexts) < 2 {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "architecture",
				Code:     "SHARED_ARCH_UNDERUSED",
				Severity: "warn",
				Message:  fmt.Sprintf("Entity '%s' is shared_arch but used by fewer than 2 bounded contexts", ent.Name),
				File:     file,
				Line:     line,
				Hint:     "Remove shared_arch or prove cross-context need via explicit read model/events.",
			}, opts)
		}
	}
}

func collectEntityContextUsage(services []normalizer.Service) map[string]map[string]struct{} {
	usage := make(map[string]map[string]struct{})
	for _, svc := range services {
		ctx := inferServiceContextName(svc.Name)
		if ctx == "admin" || ctx == "audit" {
			continue
		}
		for _, method := range svc.Methods {
			walkFlowSteps(method.Flow, func(step normalizer.FlowStep) {
				entity := strings.TrimSpace(stepEntityAccess(step))
				if entity == "" {
					return
				}
				if usage[entity] == nil {
					usage[entity] = map[string]struct{}{}
				}
				usage[entity][ctx] = struct{}{}
			})
		}
	}
	return usage
}

func walkFlowSteps(steps []normalizer.FlowStep, fn func(step normalizer.FlowStep)) {
	for _, step := range steps {
		fn(step)
		for _, key := range []string{"_do", "_then", "_else", "_ifNew", "_ifExists", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
			if nested, ok := step.Args[key].([]normalizer.FlowStep); ok && len(nested) > 0 {
				walkFlowSteps(nested, fn)
			}
		}
		if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok && len(cases) > 0 {
			for _, nested := range cases {
				walkFlowSteps(nested, fn)
			}
		}
		if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok && len(branches) > 0 {
			for _, nested := range branches {
				walkFlowSteps(nested, fn)
			}
		}
	}
}

func stepEntityAccess(step normalizer.FlowStep) string {
	switch {
	case strings.HasPrefix(step.Action, "repo.") || strings.HasPrefix(step.Action, "db."):
		if source, _ := step.Args["source"].(string); strings.TrimSpace(source) != "" {
			return source
		}
	case step.Action == "list.Enrich":
		if source, _ := step.Args["lookupSource"].(string); strings.TrimSpace(source) != "" {
			return source
		}
	}
	return ""
}

func inferServiceContextName(service string) string {
	s := strings.TrimSpace(strings.ToLower(service))
	s = strings.TrimSuffix(s, "service")
	s = strings.TrimSuffix(s, "services")
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return s
}

func isSharedArchEntity(ent normalizer.Entity) bool {
	if ent.Metadata == nil {
		return false
	}
	v, ok := ent.Metadata["shared_arch"].(bool)
	return ok && v
}

func toMetadataString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

func sortedSetKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
