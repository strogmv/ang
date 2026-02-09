package emitter

import (
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func buildPythonEntityNameSet(entities []normalizer.Entity) map[string]struct{} {
	out := make(map[string]struct{}, len(entities))
	for _, ent := range entities {
		name := ExportName(strings.TrimSpace(ent.Name))
		if name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

func pythonFieldTypeWithEntities(f normalizer.Field, entityNames map[string]struct{}) string {
	base := pythonTypeFromNormalized(f.Type, f.Name, entityNames)

	if f.IsList && !strings.HasPrefix(base, "list[") {
		itemType := f.ItemTypeName
		if strings.TrimSpace(itemType) == "" {
			itemType = f.Type
		}
		base = "list[" + pythonTypeFromNormalized(itemType, f.Name, entityNames) + "]"
	}
	if f.IsOptional {
		base += " | None"
	}
	return base
}

func pythonTypeFromNormalized(typeName, fieldName string, entityNames map[string]struct{}) string {
	t := strings.TrimSpace(typeName)
	if t == "" {
		return "Any"
	}

	for strings.HasPrefix(t, "*") {
		t = strings.TrimSpace(strings.TrimPrefix(t, "*"))
	}

	if strings.HasPrefix(t, "[]") {
		item := strings.TrimSpace(strings.TrimPrefix(t, "[]"))
		return "list[" + pythonTypeFromNormalized(item, fieldName, entityNames) + "]"
	}

	// Normalizer often collapses scalar aliases into map[string]any.
	// Prefer scalar types unless the field is likely a JSON/object payload.
	switch t {
	case "map[string]any", "map[string]interface{}", "map":
		if isLikelyJSONObjectField(fieldName) {
			return "dict[str, Any]"
		}
		if scalar := pythonScalarAliasType("", fieldName); scalar != "" {
			return scalar
		}
		return "str"
	}

	if strings.HasPrefix(t, "map[") {
		if strings.HasPrefix(t, "map[string]") {
			valType := strings.TrimPrefix(t, "map[string]")
			valType = strings.TrimSpace(valType)
			if valType == "" {
				valType = "any"
			}
			return "dict[str, " + pythonTypeFromNormalized(valType, fieldName, entityNames) + "]"
		}
		return "dict[str, Any]"
	}

	lower := strings.ToLower(t)
	switch lower {
	case "string", "text", "email", "url", "uuid":
		return "str"
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		return "int"
	case "float", "float32", "float64", "number", "decimal":
		return "float"
	case "bool", "boolean":
		return "bool"
	case "time", "datetime", "timestamp", "date", "time.time":
		return "datetime"
	case "bytes", "binary", "[]byte":
		return "bytes"
	case "json", "object":
		return "dict[str, Any]"
	case "any", "interface{}":
		if isLikelyJSONObjectField(fieldName) {
			return "dict[str, Any]"
		}
		if scalar := pythonScalarAliasType("", fieldName); scalar != "" {
			return scalar
		}
		return "str"
	}

	if strings.HasPrefix(t, "domain.") {
		name := ExportName(strings.TrimSpace(strings.TrimPrefix(t, "domain.")))
		if _, ok := entityNames[name]; ok {
			return name
		}
		if scalar := pythonScalarAliasType(name, fieldName); scalar != "" {
			return scalar
		}
		return "Any"
	}

	name := ExportName(t)
	if _, ok := entityNames[name]; ok {
		return name
	}
	if dot := strings.LastIndex(t, "."); dot >= 0 && dot < len(t)-1 {
		last := ExportName(t[dot+1:])
		if _, ok := entityNames[last]; ok {
			return last
		}
		if scalar := pythonScalarAliasType(last, fieldName); scalar != "" {
			return scalar
		}
	}

	if scalar := pythonScalarAliasType(name, fieldName); scalar != "" {
		return scalar
	}
	if isLikelyJSONObjectField(fieldName) {
		return "dict[str, Any]"
	}
	return "str"
}

func pythonScalarAliasType(typeOrAlias, fieldName string) string {
	v := strings.ToLower(strings.TrimSpace(typeOrAlias))
	f := strings.ToLower(strings.TrimSpace(fieldName))

	if strings.HasSuffix(v, "id") || strings.Contains(v, "uuid") || strings.Contains(v, "email") || strings.Contains(v, "slug") || strings.Contains(v, "url") || strings.Contains(v, "phone") || strings.Contains(v, "token") {
		return "str"
	}
	if strings.Contains(v, "time") || strings.Contains(v, "date") {
		return "datetime"
	}
	if strings.HasSuffix(f, "id") || strings.Contains(f, "uuid") || strings.Contains(f, "email") || strings.Contains(f, "slug") || strings.Contains(f, "url") || strings.Contains(f, "phone") || strings.Contains(f, "token") {
		return "str"
	}
	if strings.Contains(f, "time") || strings.Contains(f, "date") {
		return "datetime"
	}
	if strings.Contains(f, "count") || strings.Contains(f, "total") || strings.Contains(f, "size") || strings.Contains(f, "age") || strings.HasSuffix(f, "num") {
		return "int"
	}
	if strings.HasPrefix(f, "is") || strings.HasPrefix(f, "has") || strings.HasPrefix(f, "can") || strings.HasPrefix(f, "should") {
		return "bool"
	}
	return ""
}

func isLikelyJSONObjectField(fieldName string) bool {
	f := strings.ToLower(strings.TrimSpace(fieldName))
	if f == "" {
		return false
	}
	jsonish := []string{"meta", "payload", "data", "attrs", "attributes", "props", "extra", "raw", "json", "config", "settings"}
	for _, marker := range jsonish {
		if strings.Contains(f, marker) {
			return true
		}
	}
	return false
}
