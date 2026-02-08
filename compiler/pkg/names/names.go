package names

import (
	"regexp"
	"strings"
)

var (
	// Слова, которые в Go должны быть в CAPS (Go Standard Initialisms)
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
	}
)

// ToGoName преобразует строку в PascalCase с учетом Go Initialisms (userId -> UserID)
func ToGoName(s string) string {
	if s == "" {
		return ""
	}
	
	// Обработка специальных случаев
	if strings.ToLower(s) == "ids" {
		return "IDs"
	}

	parts := splitName(s)
	for i, p := range parts {
		upper := strings.ToUpper(p)
		if goInitialisms[upper] {
			parts[i] = upper
		} else {
			low := strings.ToLower(p)
			parts[i] = strings.ToUpper(low[:1]) + low[1:]
		}
	}
	return strings.Join(parts, "")
}

// ToJSONName преобразует строку в camelCase с Id (UserID -> userId)
func ToJSONName(s string) string {
	if s == "" {
		return ""
	}
	
	parts := splitName(s)
	for i, p := range parts {
		lower := strings.ToLower(p)
		if i == 0 {
			parts[i] = lower
		} else {
			// Frontend standard: use "Id" instead of "ID"
			if lower == "id" {
				parts[i] = "Id"
			} else {
				parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
			}
		}
	}
	return strings.Join(parts, "")
}

func splitName(s string) []string {
	// Разбиваем по границам слов: 
	// 1. camelCase/PascalCase
	// 2. Underscores/Numbers
	re := regexp.MustCompile("([A-Z]+[a-z0-9]*|[a-z0-9]+)")
	return re.FindAllString(s, -1)
}
