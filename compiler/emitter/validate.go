package emitter

import (
	"strings"
	"strconv"
)

type validationRules struct {
	Required bool
	Email    bool
	URL      bool
	Min      *float64
	Max      *float64
	Len      *float64
	Gte      *float64
	Lte      *float64
	Gt       *float64
	Lt       *float64
}

func parseValidateTag(tag string) validationRules {
	var rules validationRules
	parts := strings.Split(tag, ",")
	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		switch part {
		case "required":
			rules.Required = true
			continue
		case "email":
			rules.Email = true
			continue
		case "url":
			rules.URL = true
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if val == "" {
			continue
		}
		num, err := strconv.ParseFloat(val, 64)
		if err != nil {
			continue
		}
		switch key {
		case "min":
			rules.Min = &num
		case "max":
			rules.Max = &num
		case "len":
			rules.Len = &num
		case "gte":
			rules.Gte = &num
		case "lte":
			rules.Lte = &num
		case "gt":
			rules.Gt = &num
		case "lt":
			rules.Lt = &num
		}
	}
	return rules
}
