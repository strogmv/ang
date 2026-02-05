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
	for i, w := range words {
		lower := strings.ToLower(w)
		if acronym, ok := knownAcronyms[lower]; ok {
			words[i] = acronym
			continue
		}
		r := []rune(lower)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
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
