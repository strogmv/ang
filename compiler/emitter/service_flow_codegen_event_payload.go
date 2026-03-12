package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderEventPayloadExpr(st *flowRenderState, step normalizer.FlowStep, eventName string, arg func(string) string) string {
	if payloadMap := flowEventPayloadMap(step.Args["payloadMap"]); payloadMap != nil {
		if len(payloadMap) == 0 {
			return fmt.Sprintf("domain.%s{}", ExportName(eventName))
		}
		eventDef, hasEvent := normalizer.EventDef{}, false
		if st != nil && st.eventDefsByName != nil {
			eventDef, hasEvent = st.eventDefsByName[eventName]
		}
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
			v = coerceValueForEventField(k, v, eventDef, hasEvent, st)
			parts = append(parts, fmt.Sprintf("%s: %s", k, v))
		}
		return fmt.Sprintf("domain.%s{%s}", ExportName(eventName), strings.Join(parts, ", "))
	}

	payload := strings.TrimSpace(arg("payload"))
	if payload == "" {
		return fmt.Sprintf("domain.%s{}", ExportName(eventName))
	}
	return normalizePayloadExpr(payload)
}

func coerceValueForEventField(
	fieldName, valueExpr string,
	eventDef normalizer.EventDef,
	hasEvent bool,
	st *flowRenderState,
) string {
	valueExpr = strings.TrimSpace(valueExpr)
	if valueExpr == "" {
		return valueExpr
	}
	if strings.Contains(valueExpr, ".Format(") || strings.HasPrefix(valueExpr, "fmt.Sprint(") {
		return valueExpr
	}

	if !hasEvent {
		return legacyNormalizePayloadField(fieldName, valueExpr)
	}

	eventFieldType, hasField := eventFieldTypeByName(eventDef, fieldName)
	if !hasField {
		return legacyNormalizePayloadField(fieldName, valueExpr)
	}

	var sourceVarType string
	if st != nil {
		sourceVarType = resolveValueExprType(valueExpr, st.types, st.entityDefsByName)
	}

	if isTimeType(sourceVarType) && (eventFieldType == "" || isStringType(eventFieldType)) {
		return valueExpr + ".Format(time.RFC3339)"
	}
	if isNumericType(sourceVarType) && isStringType(eventFieldType) {
		return fmt.Sprintf("fmt.Sprint(%s)", valueExpr)
	}
	return valueExpr
}

func eventFieldTypeByName(eventDef normalizer.EventDef, fieldName string) (string, bool) {
	want := canonicalFlowFieldName(fieldName)
	for _, f := range eventDef.Fields {
		if canonicalFlowFieldName(f.Name) == want {
			return strings.TrimSpace(f.Type), true
		}
	}
	return "", false
}

func resolveValueExprType(expr string, varTypes map[string]string, entities map[string]normalizer.Entity) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	if isQuotedExpr(expr) {
		return "string"
	}
	if expr == "true" || expr == "false" {
		return "bool"
	}
	if basic := inferBasicNumericType(expr); basic != "" {
		return basic
	}
	if varTypes == nil {
		return ""
	}

	root, tail, ok := splitSelectorPath(expr)
	if !ok {
		return strings.TrimSpace(varTypes[expr])
	}

	current := strings.TrimSpace(varTypes[root])
	if current == "" {
		return ""
	}
	for _, segment := range tail {
		seg := strings.TrimSpace(segment)
		if seg == "" {
			return ""
		}
		current = derefType(current)
		entityName, isDomain := domainEntityTypeName(current)
		if !isDomain {
			return ""
		}
		ent, ok := entities[entityName]
		if !ok {
			return ""
		}
		fieldType, ok := entityFieldTypeByName(ent, seg)
		if !ok {
			return ""
		}
		current = fieldType
	}
	return strings.TrimSpace(current)
}

func entityFieldTypeByName(ent normalizer.Entity, fieldName string) (string, bool) {
	want := canonicalFlowFieldName(fieldName)
	for _, f := range ent.Fields {
		if canonicalFlowFieldName(f.Name) == want {
			return strings.TrimSpace(f.Type), true
		}
	}
	return "", false
}

func splitSelectorPath(expr string) (string, []string, bool) {
	if expr == "" || strings.ContainsAny(expr, " ()[]{}+-*/%<>=!&|,:") {
		return "", nil, false
	}
	parts := strings.Split(expr, ".")
	if len(parts) == 0 {
		return "", nil, false
	}
	root := strings.TrimSpace(parts[0])
	if root == "" {
		return "", nil, false
	}
	if len(parts) == 1 {
		return root, nil, true
	}
	return root, parts[1:], true
}

func derefType(t string) string {
	t = strings.TrimSpace(t)
	for strings.HasPrefix(t, "*") {
		t = strings.TrimPrefix(strings.TrimSpace(t), "*")
	}
	return t
}

func domainEntityTypeName(t string) (string, bool) {
	t = strings.TrimSpace(t)
	if strings.HasPrefix(t, "domain.") {
		name := strings.TrimSpace(strings.TrimPrefix(t, "domain."))
		return name, name != ""
	}
	return "", false
}

func isQuotedExpr(expr string) bool {
	if len(expr) < 2 {
		return false
	}
	return (strings.HasPrefix(expr, `"`) && strings.HasSuffix(expr, `"`)) ||
		(strings.HasPrefix(expr, "`") && strings.HasSuffix(expr, "`"))
}

func inferBasicNumericType(expr string) string {
	if expr == "" {
		return ""
	}
	// Fast path for literal integers/floats used in payloadMap.
	isNum := true
	hasDot := false
	for i, r := range expr {
		if i == 0 && (r == '-' || r == '+') {
			continue
		}
		if r == '.' {
			hasDot = true
			continue
		}
		if r < '0' || r > '9' {
			isNum = false
			break
		}
	}
	if !isNum {
		return ""
	}
	if hasDot {
		return "float64"
	}
	return "int"
}

func isNumericType(t string) bool {
	t = strings.TrimSpace(t)
	if t == "" {
		return false
	}
	if b, ok := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
		"float32": true, "float64": true,
	}[t]; ok {
		return b
	}
	return false
}

func isTimeType(t string) bool {
	t = derefType(t)
	return t == "time.Time"
}

func isStringType(t string) bool {
	t = derefType(strings.TrimSpace(t))
	return t == "string"
}

func canonicalFlowFieldName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return strings.ToLower(s)
}

func legacyNormalizePayloadField(fieldName, valueExpr string) string {
	fieldName = strings.TrimSpace(fieldName)
	valueExpr = strings.TrimSpace(valueExpr)
	if strings.HasSuffix(fieldName, "At") && strings.HasSuffix(valueExpr, "At") {
		return valueExpr + ".Format(time.RFC3339)"
	}
	return valueExpr
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
