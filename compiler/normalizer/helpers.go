package normalizer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"cuelang.org/go/cue"
)

func formatPos(v cue.Value) string {
	pos := v.Pos()
	if !pos.IsValid() {
		return ""
	}
	file := pos.Filename()
	line := pos.Line()

	// Try to make file path relative to current working directory
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, file); err == nil && !strings.HasPrefix(rel, "..") {
			file = rel
		}
	}

	return fmt.Sprintf("%s:%d", file, line)
}

func getString(v cue.Value, path string) string {
	res := v.LookupPath(cue.ParsePath(path))
	s, _ := res.String()
	return strings.TrimSpace(s)
}

func cleanName(s string) string {
	s = strings.TrimSuffix(s, "?")
	s = strings.TrimSuffix(s, "!")
	return strings.TrimSpace(s)
}

func normalizeServiceName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

func exportName(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func isExportedName(s string) bool {
	if s == "" {
		return false
	}
	r := []rune(s)
	return unicode.IsUpper(r[0])
}

func parseAttributes(v cue.Value) []Attribute {
	var result []Attribute
	attrs := v.Attributes(cue.ValueAttr | cue.FieldAttr | cue.DeclAttr)
	for _, attr := range attrs {
		name := attr.Name()
		args := make(map[string]any)
		for i := 0; i < attr.NumArgs(); i++ {
			k, val := attr.Arg(i)
			if k == "" {
				// For attributes like @owner(user), 'user' is an unkeyed arg
				args["_"] = val
			} else {
				args[k] = val
			}
		}
		result = append(result, Attribute{Name: name, Args: args})
	}
	return result
}

// parseSize converts size strings like "1mb", "512kb" to bytes.
func parseSize(s string) int64 {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0
	}

	var multiplier int64 = 1
	var numStr string

	if strings.HasSuffix(s, "kb") {
		multiplier = 1024
		numStr = strings.TrimSuffix(s, "kb")
	} else if strings.HasSuffix(s, "mb") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(s, "mb")
	} else if strings.HasSuffix(s, "gb") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(s, "gb")
	} else if strings.HasSuffix(s, "b") {
		numStr = strings.TrimSuffix(s, "b")
	} else {
		numStr = s
	}

	var val int64
	_, _ = fmt.Sscanf(numStr, "%d", &val) // ignore parse errors, val defaults to 0
	return val * multiplier
}

// detectType tries to infer the Go type from a CUE value.
func (n *Normalizer) detectType(fieldName string, v cue.Value) string {
	// Field definitions are usually structs with a nested `type` property.
	// Resolve declared type first to avoid degrading everything to map[string]any.
	if v.IncompleteKind() == cue.StructKind {
		typeVal := v.LookupPath(cue.ParsePath("type"))
		if typeVal.Exists() {
			if s, err := typeVal.String(); err == nil {
				if mapped := mapDeclaredCueTypeToGo(strings.TrimSpace(s)); mapped != "" {
					return mapped
				}
			}
		}
	}

	// 1. Check if type is explicitly mapped in Codegen CUE
	_, path := v.ReferencePath()
	pathStr := path.String()

	if n.TypeMapping != nil {
		if cfg, ok := n.TypeMapping[pathStr]; ok {
			return cfg.GoType
		}
		// Try short name if path is just a scalar type
		if cfg, ok := n.TypeMapping[fmt.Sprint(v.IncompleteKind())]; ok {
			return cfg.GoType
		}
	}

	// Contextual Hinting
	if fieldName == "permissions" {
		return "[]string"
	}

	// Entity Reference Check
	if v.IncompleteKind() == cue.StructKind {
		_, path := v.ReferencePath()
		if len(path.Selectors()) > 0 {
			last := path.Selectors()[len(path.Selectors())-1].String()
			if strings.HasPrefix(last, "#") {
				return "domain." + exportName(strings.TrimPrefix(last, "#"))
			}
			if len(path.Selectors()) == 1 && strings.EqualFold(fieldName, "data") && isExportedName(last) {
				return "domain." + exportName(last)
			}
			if len(path.Selectors()) >= 2 {
				prev := path.Selectors()[len(path.Selectors())-2].String()
				if prev == "domain" {
					return "domain." + exportName(last)
				}
			}
		}
	}

	switch v.IncompleteKind() {
	case cue.StringKind:
		return "string"
	case cue.IntKind:
		return "int"
	case cue.FloatKind, cue.NumberKind:
		return "float64"
	case cue.BoolKind:
		return "bool"
	case cue.ListKind:
		// Check for [...#Entity] using AnyIndex
		anyElem := v.LookupPath(cue.MakePath(cue.AnyIndex))
		if anyElem.Exists() && anyElem.IncompleteKind() == cue.StructKind {
			_, path := anyElem.ReferencePath()
			if len(path.Selectors()) > 0 {
				last := path.Selectors()[len(path.Selectors())-1].String()
				if strings.HasPrefix(last, "#") {
					return "[]domain." + exportName(strings.TrimPrefix(last, "#"))
				}
				if len(path.Selectors()) == 1 && strings.EqualFold(fieldName, "data") && isExportedName(last) {
					return "[]domain." + exportName(last)
				}
				if len(path.Selectors()) >= 2 {
					prev := path.Selectors()[len(path.Selectors())-2].String()
					if prev == "domain" {
						return "[]domain." + exportName(last)
					}
				}
			}
		}
		// Handle scalar alias references and enum/string list aliases.
		if anyElem.Exists() && anyElem.IncompleteKind() == cue.StringKind {
			return "[]string"
		}

		strRep := fmt.Sprint(v)
		if strings.Contains(strRep, "#") {
			// Very basic extraction of #Entity from [...#Entity]
			parts := strings.Split(strRep, "#")
			if len(parts) > 1 {
				entityName := ""
				for _, r := range parts[1] {
					if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
						entityName += string(r)
					} else {
						break
					}
				}
				if entityName != "" {
					return "[]domain." + entityName
				}
			}
		}

		// Attempt to inspect list element type (concrete)
		iter, _ := v.List()
		isString := true
		count := 0
		for iter.Next() {
			count++
			if iter.Value().IncompleteKind() != cue.StringKind {
				isString = false
				break
			}
		}
		if count > 0 && isString {
			return "[]string"
		}

		// HACK for Prototype:
		// If the value prints as `[...string]`, use []string
		if strings.Contains(strRep, "string") && strings.HasPrefix(strRep, "[") {
			return "[]string"
		}
		// Enum-like unions of string literals (e.g. ["email"|"sms"]) should stay []string.
		if strings.HasPrefix(strings.TrimSpace(strRep), "[") && strings.Contains(strRep, "\"") && strings.Contains(strRep, "|") {
			return "[]string"
		}

		return "[]any"
	case cue.StructKind:
		return "map[string]any"
	default:
		return "any"
	}
}

func mapDeclaredCueTypeToGo(raw string) string {
	t := strings.Trim(strings.TrimSpace(raw), "\"")
	if t == "" {
		return ""
	}
	for strings.HasPrefix(t, "*") {
		t = strings.TrimPrefix(t, "*")
	}
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}

	// CUE unions of string literals are enums in practice.
	if strings.Contains(t, "|") {
		return "string"
	}
	if strings.HasPrefix(t, "[]") {
		if item := mapDeclaredCueTypeToGo(strings.TrimPrefix(t, "[]")); item != "" {
			return "[]" + strings.TrimPrefix(item, "[]")
		}
		return "[]any"
	}

	switch strings.ToLower(t) {
	case "string", "email", "url", "phone", "password", "uuid":
		return "string"
	case "int", "int32":
		return "int"
	case "int64":
		return "int64"
	case "float", "float32", "float64", "number":
		return "float64"
	case "bool", "boolean":
		return "bool"
	case "time", "time.time", "datetime":
		return "time.Time"
	case "json", "object", "map":
		return "map[string]any"
	case "money":
		return "int64"
	default:
		return ""
	}
}
