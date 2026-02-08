package emitter

import (
	"strings"
	"unicode"
)

func HumanizeName(name string) string {
	if name == "" {
		return ""
	}

	normalized := ExportName(name)
	words := splitPascalWords(normalized)
	// Capitalize each word properly
	for i, w := range words {
		low := strings.ToLower(w)
		words[i] = strings.ToUpper(low[:1]) + low[1:]
	}

	return strings.Join(words, " ")
}

func splitPascalWords(s string) []string {
	if s == "" {
		return nil
	}

	runes := []rune(s)
	var words []string
	start := 0

	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		curr := runes[i]
		nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])

		if unicode.IsUpper(curr) && (unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextLower)) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}

	words = append(words, string(runes[start:]))
	return words
}
