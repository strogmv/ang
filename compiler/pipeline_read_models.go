package compiler

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func emitReadModelDiagnostics(entities []normalizer.Entity, events []normalizer.EventDef, opts PipelineOptions) {
	eventSet := make(map[string]struct{}, len(events))
	for _, evt := range events {
		eventSet[strings.TrimSpace(evt.Name)] = struct{}{}
	}

	normalizeContext := func(v string) string {
		v = strings.TrimSpace(strings.ToLower(v))
		if v == "" {
			return ""
		}
		for _, sep := range []string{"_", "-", "."} {
			if i := strings.Index(v, sep); i > 0 {
				return v[:i]
			}
		}
		return v
	}

	for _, ent := range entities {
		rm := ent.ReadModel
		if rm == nil {
			if b, ok := ent.Metadata["read_model"].(bool); !ok || !b {
				continue
			}
			rm = &normalizer.ReadModelDef{}
			if src, ok := ent.Metadata["read_model_source_context"].(string); ok {
				rm.SourceContext = strings.TrimSpace(strings.ToLower(src))
			}
			switch v := ent.Metadata["read_model_refresh_on"].(type) {
			case []string:
				rm.RefreshOn = append([]string(nil), v...)
			case []any:
				for _, item := range v {
					s := strings.TrimSpace(fmt.Sprint(item))
					if s != "" {
						rm.RefreshOn = append(rm.RefreshOn, s)
					}
				}
			}
		}

		file, line := parseSourcePos(ent.Source)
		srcCtx := normalizeContext(rm.SourceContext)
		entityCtx := normalizeContext(ent.BoundedContext)
		if entityCtx == "" {
			entityCtx = normalizeContext(ent.Owner)
		}

		if srcCtx == "" {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "read-model",
				Code:     "READ_MODEL_SOURCE_CONTEXT_MISSING",
				Severity: "error",
				Message:  fmt.Sprintf("Read model %s must declare read_model.source_context", ent.Name),
				Hint:     "Set read_model: { source_context: \"company\", refreshOn: [\"EventName\"] }",
				File:     file,
				Line:     line,
			}, opts)
		}
		if len(rm.RefreshOn) == 0 {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "read-model",
				Code:     "READ_MODEL_REFRESH_ON_MISSING",
				Severity: "error",
				Message:  fmt.Sprintf("Read model %s must declare read_model.refreshOn events", ent.Name),
				Hint:     "Add read_model.refreshOn with at least one domain event name",
				File:     file,
				Line:     line,
			}, opts)
			continue
		}
		for _, evt := range rm.RefreshOn {
			evt = strings.TrimSpace(evt)
			if evt == "" {
				continue
			}
			if _, ok := eventSet[evt]; ok {
				continue
			}
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "read-model",
				Code:     "READ_MODEL_REFRESH_EVENT_UNKNOWN",
				Severity: "error",
				Message:  fmt.Sprintf("Read model %s references unknown refresh event %s", ent.Name, evt),
				Hint:     "Define this event in cue/events or fix the refreshOn name",
				File:     file,
				Line:     line,
			}, opts)
		}
		if srcCtx != "" && entityCtx != "" && srcCtx == entityCtx {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "read-model",
				Code:     "READ_MODEL_SAME_CONTEXT",
				Severity: "warn",
				Message:  fmt.Sprintf("Read model %s source_context equals bounded context (%s)", ent.Name, srcCtx),
				Hint:     "Use read model for cross-context data; same-context projection is usually unnecessary",
				File:     file,
				Line:     line,
			}, opts)
		}
	}
}
