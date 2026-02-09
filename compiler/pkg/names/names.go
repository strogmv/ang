package names

import (
	"regexp"
	"strings"
)

var (
	// Words that should stay uppercased in Go identifiers.
	goInitialisms = map[string]bool{
		"ID":    true,
		"API":   true,
		"HTTP":  true,
		"HTTPS": true,
		"IP":    true,
		"JSON":  true,
		"SQL":   true,
		"TTL":   true,
		"UUID":  true,
		"URI":   true,
		"URL":   true,
		"TCP":   true,
		"UDP":   true,
		"FSM":   true,
		"DTO":   true,
		"UI":    true,
		"IO":    true,
		"S3":    true,
		"JWT":   true,
		"RPC":   true,
		"AI":    true,
	}

	reAcronymBoundary = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	reCamelBoundary   = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	reToken           = regexp.MustCompile(`[A-Za-z0-9]+`)
)

// ToGoName converts a name to PascalCase with Go initialism normalization.
func ToGoName(s string) string {
	if s == "" {
		return ""
	}

	// Special cases.
	if strings.ToLower(s) == "ids" {
		return "IDs"
	}
	if strings.EqualFold(s, "apikey") {
		return "APIKey"
	}
	if strings.EqualFold(s, "apikeys") {
		return "APIKeys"
	}

	parts := splitName(s)
	for i, p := range parts {
		upper := strings.ToUpper(p)
		if goInitialisms[upper] {
			parts[i] = upper
			continue
		}
		low := strings.ToLower(p)
		parts[i] = strings.ToUpper(low[:1]) + low[1:]
	}
	return strings.Join(parts, "")
}

// ToJSONName converts a name to camelCase using frontend conventions (Id over ID).
func ToJSONName(s string) string {
	if s == "" {
		return ""
	}

	parts := splitName(s)
	for i, p := range parts {
		lower := strings.ToLower(p)
		if i == 0 {
			parts[i] = lower
			continue
		}
		if lower == "id" {
			parts[i] = "Id"
			continue
		}
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, "")
}

func splitName(s string) []string {
	if s == "" {
		return nil
	}
	clean := strings.NewReplacer("_", " ", "-", " ", ".", " ", "/", " ").Replace(s)
	clean = reAcronymBoundary.ReplaceAllString(clean, `${1} ${2}`)
	clean = reCamelBoundary.ReplaceAllString(clean, `${1} ${2}`)

	tokens := reToken.FindAllString(clean, -1)
	if len(tokens) == 0 {
		return []string{s}
	}
	return tokens
}
