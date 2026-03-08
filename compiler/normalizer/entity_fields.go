package normalizer

import (
	"fmt"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
)

func (n *Normalizer) parseInlineFields(val cue.Value) ([]Field, error) {
	var fields []Field
	iter, err := val.Fields(cue.All())
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		fLabel := cleanName(iter.Selector().String())
		if fLabel == "" {
			continue
		}
		fVal := iter.Value()
		var defVal string
		dVal, _ := fVal.Default()
		if dVal.IsConcrete() && (dVal.IncompleteKind() != cue.StructKind && dVal.IncompleteKind() != cue.ListKind) {
			defVal = fmt.Sprint(dVal)
		}
		field := Field{
			Name:        fLabel,
			IsOptional:  iter.IsOptional(),
			Type:        n.detectType(fLabel, fVal),
			Default:     defVal,
			DB:          parseDBTags(fVal),
			ValidateTag: inferValidatorTags(fLabel, fVal),
			Constraints: extractConstraints(fVal),
			EnvVar:      parseEnvTag(fVal),
			UI:          parseUIHints(fVal),
			Source:      formatPos(fVal),
		}

		if attr := fVal.Attribute("secret"); attr.Err() == nil {
			field.IsSecret = true
		}
		// Note: no auto-detect by field name here; parseInlineFields is used
		// for nested data fields inside operations where password/token fields
		// are part of the API contract. Use @secret explicitly if needed.
		if attr := fVal.Attribute("pii"); attr.Err() == nil {
			field.IsPII = true
		}
		if attr := fVal.Attribute("encrypt"); attr.Err() == nil {
			if field.Metadata == nil {
				field.Metadata = make(map[string]any)
			}
			mode := "randomized"
			if m, found, _ := attr.Lookup(0, "mode"); found {
				mode = m
			}
			field.Metadata["encrypt"] = mode
		}
		if attr := fVal.Attribute("redact"); attr.Err() == nil {
			if field.Metadata == nil {
				field.Metadata = make(map[string]any)
			}
			field.Metadata["redact"] = true
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func parseEnvTag(v cue.Value) string {
	attr := v.Attribute("env")
	if attr.Err() == nil {
		val := attr.Contents()
		return strings.Trim(val, "\"")
	}
	return ""
}

func inferValidatorTags(name string, v cue.Value) string {
	// 1. Explicit tag (@validate)
	attrs := v.Attributes(cue.ValueAttr)
	for _, attr := range attrs {
		if attr.Name() == "validate" {
			val := attr.Contents()
			val = strings.TrimPrefix(val, "rule=")
			val = strings.Trim(val, "\"")
			return val
		}
	}

	// 2. Heuristic (Auto-Discovery)
	name = strings.ToLower(name)
	if name == "email" {
		return "email"
	}
	if strings.Contains(name, "url") {
		return "url"
	}

	return ""
}

func parseDBTags(v cue.Value) DBMeta {
	meta := DBMeta{
		Type: "TEXT", // Default
	}

	attr := v.Attribute("db")
	if err := attr.Err(); err != nil {
		return meta
	}

	if val, found, _ := attr.Lookup(0, "type"); found {
		meta.Type = val
	}

	if _, found, _ := attr.Lookup(0, "primary_key"); found {
		meta.PrimaryKey = true
	}

	if _, found, _ := attr.Lookup(0, "unique"); found {
		meta.Unique = true
	}

	if _, found, _ := attr.Lookup(0, "index"); found {
		meta.Index = true
	}

	return meta
}

func parseUIHints(v cue.Value) *UIHints {
	attr := v.Attribute("ui")
	if err := attr.Err(); err != nil {
		return nil
	}

	hints := &UIHints{
		FullWidth: true, // default
	}

	if val, found, _ := attr.Lookup(0, "type"); found {
		hints.Type = val
	}
	if val, found, _ := attr.Lookup(0, "importance"); found {
		hints.Importance = val
	}
	if val, found, _ := attr.Lookup(0, "inputKind"); found {
		hints.InputKind = val
	}
	if val, found, _ := attr.Lookup(0, "intent"); found {
		hints.Intent = val
	}
	if val, found, _ := attr.Lookup(0, "density"); found {
		hints.Density = val
	}
	if val, found, _ := attr.Lookup(0, "labelMode"); found {
		hints.LabelMode = val
	}
	if val, found, _ := attr.Lookup(0, "surface"); found {
		hints.Surface = val
	}
	if val, found, _ := attr.Lookup(0, "component"); found {
		hints.Component = val
	}
	if val, found, _ := attr.Lookup(0, "section"); found {
		hints.Section = val
	}
	if val, found, _ := attr.Lookup(0, "columns"); found {
		if n, err := strconv.Atoi(val); err == nil {
			hints.Columns = n
		}
	}
	if val, found, _ := attr.Lookup(0, "label"); found {
		hints.Label = val
	}
	if val, found, _ := attr.Lookup(0, "placeholder"); found {
		hints.Placeholder = val
	}
	if val, found, _ := attr.Lookup(0, "helperText"); found {
		hints.HelperText = val
	}
	if val, found, _ := attr.Lookup(0, "order"); found {
		if n, err := strconv.Atoi(val); err == nil {
			hints.Order = n
		}
	}
	if _, found, _ := attr.Lookup(0, "hidden"); found {
		hints.Hidden = true
	}
	if _, found, _ := attr.Lookup(0, "disabled"); found {
		hints.Disabled = true
	}
	if val, found, _ := attr.Lookup(0, "fullWidth"); found {
		hints.FullWidth = val != "false"
	}
	if val, found, _ := attr.Lookup(0, "rows"); found {
		if n, err := strconv.Atoi(val); err == nil {
			hints.Rows = n
		}
	}
	if val, found, _ := attr.Lookup(0, "min"); found {
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			hints.Min = &n
		}
	}
	if val, found, _ := attr.Lookup(0, "max"); found {
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			hints.Max = &n
		}
	}
	if val, found, _ := attr.Lookup(0, "step"); found {
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			hints.Step = &n
		}
	}
	if val, found, _ := attr.Lookup(0, "currency"); found {
		hints.Currency = val
	}
	if val, found, _ := attr.Lookup(0, "source"); found {
		hints.Source = val
	}
	if val, found, _ := attr.Lookup(0, "accept"); found {
		hints.Accept = val
	}
	if val, found, _ := attr.Lookup(0, "maxSize"); found {
		if n, err := strconv.Atoi(val); err == nil {
			hints.MaxSize = n
		}
	}
	if _, found, _ := attr.Lookup(0, "multiple"); found {
		hints.Multiple = true
	}

	return hints
}

func extractConstraints(v cue.Value) *Constraints {
	c := &Constraints{}
	hasAny := false

	op, args := v.Expr()

	// Recursively handle AND (e.g. >0 & <100)
	if op == cue.AndOp {
		for _, arg := range args {
			sub := extractConstraints(arg)
			if sub != nil {
				if sub.Min != nil {
					c.Min = sub.Min
				}
				if sub.Max != nil {
					c.Max = sub.Max
				}
				if sub.MinLen != nil {
					c.MinLen = sub.MinLen
				}
				if sub.MaxLen != nil {
					c.MaxLen = sub.MaxLen
				}
				if sub.Regex != "" {
					c.Regex = sub.Regex
				}
				if len(sub.Enum) > 0 {
					c.Enum = sub.Enum
				}
				hasAny = true
			}
		}
	}

	switch op {
	case cue.GreaterThanOp, cue.GreaterThanEqualOp:
		val, _ := args[0].Float64()
		c.Min = &val
		hasAny = true
	case cue.LessThanOp, cue.LessThanEqualOp:
		val, _ := args[0].Float64()
		c.Max = &val
		hasAny = true
	case cue.CallOp:
		// Handle built-ins like strings.MinRunes(5)
		name := fmt.Sprint(args[0])
		if strings.Contains(name, "MinRunes") {
			v, _ := args[1].Int64()
			iv := int(v)
			c.MinLen = &iv
			hasAny = true
		} else if strings.Contains(name, "MaxRunes") {
			v, _ := args[1].Int64()
			iv := int(v)
			c.MaxLen = &iv
			hasAny = true
		}
	case cue.OrOp:
		// Handle enums: "a" | "b" | "c"
		isEnum := true
		var enum []string
		for _, arg := range args {
			if s, err := arg.String(); err == nil {
				enum = append(enum, s)
			} else {
				isEnum = false
				break
			}
		}
		if isEnum && len(enum) > 0 {
			c.Enum = enum
			hasAny = true
		}
	}

	if !hasAny {
		return nil
	}
	return c
}
