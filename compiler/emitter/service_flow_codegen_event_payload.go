package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderEventPayloadExpr(step normalizer.FlowStep, eventName string, arg func(string) string) string {
	if payloadMap := flowEventPayloadMap(step.Args["payloadMap"]); len(payloadMap) > 0 {
		keys := make([]string, 0, len(payloadMap))
		for k := range payloadMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			v := strings.TrimSpace(payloadMap[k])
			if v == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s: %s", k, v))
		}
		return fmt.Sprintf("domain.%s{%s}", ExportName(eventName), strings.Join(parts, ", "))
	}

	payload := strings.TrimSpace(arg("payload"))
	if payload == "" {
		return ""
	}
	return normalizePayloadExpr(payload)
}

func flowEventPayloadMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	switch raw := v.(type) {
	case map[string]string:
		out := make(map[string]string, len(raw))
		for k, val := range raw {
			out[k] = val
		}
		return out
	case map[string]any:
		out := make(map[string]string, len(raw))
		for k, val := range raw {
			if s, ok := val.(string); ok {
				out[k] = s
				continue
			}
			out[k] = fmt.Sprint(val)
		}
		return out
	default:
		return nil
	}
}
