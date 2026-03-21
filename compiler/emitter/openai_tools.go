package emitter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

type openAIToolSpec struct {
	ToolName     string
	OriginalName string
	ServiceName  string
	Method       normalizer.Method
	RequestType  string
	ResponseType string
	SchemaJSON   string
}

func resolveOpenAITools(st *flowRenderState, serviceName string, names []string) ([]openAIToolSpec, error) {
	if st == nil || len(names) == 0 {
		return nil, nil
	}
	svc, ok := findOpenAIService(st.serviceDefsByName, serviceName)
	if !ok {
		return nil, fmt.Errorf("openai.Chat tools require service catalog for %q", serviceName)
	}
	specs := make([]openAIToolSpec, 0, len(names))
	seenOriginal := map[string]bool{}
	seenToolName := map[string]bool{}
	for _, rawName := range names {
		rawName = strings.TrimSpace(rawName)
		if rawName == "" || seenOriginal[rawName] {
			continue
		}
		seenOriginal[rawName] = true

		targetServiceName, methodName := parseOpenAIToolRef(serviceName, rawName)
		if methodName == "" {
			return nil, fmt.Errorf("openai.Chat tool %q is invalid", rawName)
		}
		targetService, ok := findOpenAIService(st.serviceDefsByName, targetServiceName)
		if !ok {
			return nil, fmt.Errorf("openai.Chat tool %q references unknown service %q", rawName, targetServiceName)
		}
		if !strings.EqualFold(strings.TrimSpace(targetService.Name), strings.TrimSpace(svc.Name)) && !openAIToolServiceAllowed(svc, targetService.Name) {
			return nil, fmt.Errorf("openai.Chat tool %q references service %q outside %q dependencies", rawName, targetService.Name, svc.Name)
		}
		m, ok := findOpenAIMethod(targetService, methodName)
		if !ok {
			return nil, fmt.Errorf("openai.Chat tool %q not found in service %q", rawName, targetService.Name)
		}
		schemaJSON, err := buildOpenAIToolSchemaJSON(m.Input)
		if err != nil {
			return nil, fmt.Errorf("openai.Chat tool %q schema: %w", rawName, err)
		}
		toolName := openAIToolFunctionName(rawName, targetService.Name, m.Name, svc.Name)
		if seenToolName[toolName] {
			return nil, fmt.Errorf("openai.Chat tool %q collides with generated tool name %q", rawName, toolName)
		}
		seenToolName[toolName] = true
		specs = append(specs, openAIToolSpec{
			ToolName:     toolName,
			OriginalName: rawName,
			ServiceName:  targetService.Name,
			Method:       m,
			RequestType:  "port." + ExportName(m.Name) + "Request",
			ResponseType: "port." + ExportName(m.Name) + "Response",
			SchemaJSON:   schemaJSON,
		})
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].ToolName < specs[j].ToolName })
	return specs, nil
}

func findOpenAIService(services map[string]normalizer.Service, name string) (normalizer.Service, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return normalizer.Service{}, false
	}
	if svc, ok := services[name]; ok {
		return svc, true
	}
	for _, svc := range services {
		if strings.EqualFold(strings.TrimSpace(svc.Name), name) {
			return svc, true
		}
	}
	return normalizer.Service{}, false
}

func findOpenAIMethod(svc normalizer.Service, name string) (normalizer.Method, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return normalizer.Method{}, false
	}
	for _, m := range svc.Methods {
		if strings.EqualFold(strings.TrimSpace(m.Name), name) {
			return m, true
		}
	}
	return normalizer.Method{}, false
}

func parseOpenAIToolRef(currentService, raw string) (serviceName, methodName string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if parts := strings.SplitN(raw, ".", 2); len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(currentService), raw
}

func openAIToolServiceAllowed(current normalizer.Service, targetService string) bool {
	for _, dep := range current.Uses {
		if strings.EqualFold(strings.TrimSpace(dep), strings.TrimSpace(targetService)) {
			return true
		}
	}
	return false
}

func openAIToolFunctionName(original, serviceName, methodName, currentService string) string {
	if strings.Contains(strings.TrimSpace(original), ".") || !strings.EqualFold(strings.TrimSpace(serviceName), strings.TrimSpace(currentService)) {
		return ExportName(serviceName) + "__" + ExportName(methodName)
	}
	return ExportName(methodName)
}

func normalizeOpenAIToolChoiceExpr(expr string, specs []openAIToolSpec) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return expr
	}
	if !strings.HasPrefix(expr, `"`) || !strings.HasSuffix(expr, `"`) {
		return expr
	}
	raw, err := strconv.Unquote(expr)
	if err != nil {
		return expr
	}
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "auto", "none", "required":
		return expr
	}
	for _, spec := range specs {
		if raw == spec.OriginalName || raw == spec.ToolName || raw == spec.Method.Name {
			return strconv.Quote(spec.ToolName)
		}
	}
	return expr
}

func buildOpenAIToolSchemaJSON(ent normalizer.Entity) (string, error) {
	properties := map[string]any{}
	required := make([]string, 0, len(ent.Fields))
	for _, field := range ent.Fields {
		fieldName := strings.TrimSpace(field.Name)
		if fieldName == "" {
			continue
		}
		properties[fieldName] = map[string]any{
			"type": openAIJSONType(field),
		}
		if !field.IsOptional {
			required = append(required, fieldName)
		}
	}
	sort.Strings(required)
	doc := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		doc["required"] = required
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func openAIJSONType(field normalizer.Field) string {
	base := strings.TrimSpace(strings.ToLower(field.Type))
	switch {
	case strings.HasPrefix(base, "[]"):
		return "array"
	case base == "int" || base == "int32" || base == "int64" || base == "uint" || base == "uint32" || base == "uint64":
		return "integer"
	case base == "float" || base == "float32" || base == "float64" || base == "decimal":
		return "number"
	case base == "bool" || base == "boolean":
		return "boolean"
	case strings.HasPrefix(base, "map["):
		return "object"
	default:
		return "string"
	}
}

func parseOpenAIToolNames(step normalizer.FlowStep) []string {
	if step.Args == nil {
		return nil
	}
	raw, ok := step.Args["tools"]
	if !ok {
		return nil
	}
	switch x := raw.(type) {
	case []string:
		out := make([]string, 0, len(x))
		for _, s := range x {
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, t)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}
