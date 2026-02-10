package planner

import (
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/pkg/names"
)

func BuildModelPlans(entities []ir.Entity) []ModelPlan {
	entityNames := make(map[string]struct{}, len(entities))
	for _, ent := range entities {
		entityNames[ent.Name] = struct{}{}
	}

	out := make([]ModelPlan, 0, len(entities))
	for _, ent := range entities {
		m := ModelPlan{Name: names.ToGoName(ent.Name)}
		for _, f := range ent.Fields {
			if f.SkipDomain {
				continue
			}
			typ := pythonTypeFromIRField(f, entityNames)
			if f.Optional && !strings.Contains(typ, "| None") {
				typ += " | None"
			}
			m.Fields = append(m.Fields, FieldPlan{
				Name:       f.Name,
				Type:       typ,
				IsOptional: f.Optional,
			})
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func pythonTypeFromIRTypeRef(t ir.TypeRef, entityNames map[string]struct{}) string {
	switch t.Kind {
	case ir.KindString:
		return "str"
	case ir.KindInt, ir.KindInt64:
		return "int"
	case ir.KindFloat:
		return "float"
	case ir.KindBool:
		return "bool"
	case ir.KindTime:
		return "datetime"
	case ir.KindUUID:
		return "str"
	case ir.KindJSON:
		return "dict[str, Any]"
	case ir.KindFile:
		return "str"
	case ir.KindEnum:
		return "str"
	case ir.KindAny:
		return "Any"
	case ir.KindEntity:
		if t.Name != "" {
			if _, ok := entityNames[t.Name]; ok {
				return names.ToGoName(t.Name)
			}
			if scalar := pythonScalarAliasType(t.Name, ""); scalar != "" {
				return scalar
			}
			return "Any"
		}
		return "Any"
	case ir.KindList:
		if t.ItemType == nil {
			return "list[Any]"
		}
		return "list[" + pythonTypeFromIRTypeRef(*t.ItemType, entityNames) + "]"
	case ir.KindMap:
		key := "str"
		if t.KeyType != nil {
			key = pythonTypeFromIRTypeRef(*t.KeyType, entityNames)
		}
		val := "Any"
		if t.ItemType != nil {
			val = pythonTypeFromIRTypeRef(*t.ItemType, entityNames)
		}
		return "dict[" + key + ", " + val + "]"
	default:
		return "Any"
	}
}

func pythonTypeFromIRField(f ir.Field, entityNames map[string]struct{}) string {
	// IR conversion can collapse unknown/map-like aliases into KindAny.
	// Use field-name heuristics to keep backward-compatible Python model typing.
	if f.Type.Kind == ir.KindAny {
		if isLikelyJSONObjectField(f.Name) {
			return "dict[str, Any]"
		}
		if scalar := pythonScalarAliasType("", f.Name); scalar != "" {
			return scalar
		}
		return "Any"
	}
	if f.Type.Kind == ir.KindEntity && f.Type.Name != "" {
		if _, ok := entityNames[f.Type.Name]; ok {
			return names.ToGoName(f.Type.Name)
		}
		if scalar := pythonScalarAliasType(f.Type.Name, f.Name); scalar != "" {
			return scalar
		}
	}
	return pythonTypeFromIRTypeRef(f.Type, entityNames)
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
