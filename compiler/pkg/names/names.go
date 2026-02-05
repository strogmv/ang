package names

import (
	"strings"
	"unicode"
)

// KnownAcronyms are common abbreviations that should stay uppercase
var KnownAcronyms = map[string]string{
	"id": "ID", "api": "API", "jwt": "JWT", "ttl": "TTL",
	"url": "URL", "http": "HTTP", "https": "HTTPS", "sql": "SQL",
	"json": "JSON", "xml": "XML", "html": "HTML", "css": "CSS",
	"uuid": "UUID", "uri": "URI", "ip": "IP", "tcp": "TCP", "udp": "UDP",
	"fsm": "FSM", "dto": "DTO", "dao": "DAO", "orm": "ORM",
	"ai": "AI", "ml": "ML", "ui": "UI", "io": "IO", "s3": "S3",
	"vat": "VAT", "sku": "SKU", "db": "DB", "rpc": "RPC",
}

// KnownCompounds are common compound words that should be split
var KnownCompounds = map[string]string{
	"apikey":        "APIKey",
	"userid":        "UserID",
	"companyid":     "CompanyID",
	"tenderid":      "TenderID",
	"bidid":         "BidID",
	"applicationid": "ApplicationID",
	"taxid":         "TaxID",
	"vatid":         "VATID",
	"roleid":        "RoleID",
	"contactid":     "ContactID",
	"avatarlink":    "AvatarLink",
	"urlthumb":      "URLThumb",
}

// ExportName converts internal names to Go-exported names (PascalCase with ID normalization).
func ExportName(name string) string {
	if name == "" {
		return ""
	}

	lower := strings.ToLower(name)
	if compound, ok := KnownCompounds[lower]; ok {
		return compound
	}

	runes := []rune(name)
	var words []string
	var current strings.Builder

	for i, r := range runes {
		if r == '_' || r == '.' || r == '-' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			continue
		}

		if i > 0 && unicode.IsUpper(r) && unicode.IsLower(runes[i-1]) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}

		if i > 1 && unicode.IsUpper(runes[i-1]) && unicode.IsLower(r) && unicode.IsUpper(runes[i-2]) {
			str := current.String()
			if len(str) > 1 {
				words = append(words, str[:len(str)-1])
				current.Reset()
				current.WriteRune(runes[i-1])
			}
		}

		current.WriteRune(r)
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	for i, w := range words {
		lower := strings.ToLower(w)
		if acronym, ok := KnownAcronyms[lower]; ok {
			words[i] = acronym
		} else {
			r := []rune(lower)
			r[0] = unicode.ToUpper(r[0])
			words[i] = string(r)
		}
	}

	return strings.Join(words, "")
}

func ToJSONName(name string) string {
	if name == "" {
		return ""
	}

	pascal := ExportName(name)
	if pascal == "" {
		return ""
	}

	words := splitPascalWords(pascal)
	if len(words) == 0 {
		return ""
	}

	// First word: full lowercase.
	out := strings.ToLower(words[0])
	for _, w := range words[1:] {
		if w == "" {
			continue
		}
		if isAllUpper(w) {
			// Acronym in middle: "ID" -> "Id", "URL" -> "Url".
			lower := strings.ToLower(w)
			out += strings.ToUpper(lower[:1]) + lower[1:]
			continue
		}
		out += strings.ToUpper(w[:1]) + w[1:]
	}

	return out
}

func splitPascalWords(s string) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	var words []string
	var current strings.Builder

	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			var next rune
			hasNext := i+1 < len(runes)
			if hasNext {
				next = runes[i+1]
			}

			// Boundary between lower and upper: "companyID" -> "company", "ID"
			if unicode.IsUpper(r) && unicode.IsLower(prev) {
				words = appendIfNonEmpty(words, &current)
			} else if hasNext && unicode.IsUpper(prev) && unicode.IsUpper(r) && unicode.IsLower(next) {
				// Boundary before last upper when next is lower: "URLPath" -> "URL", "Path"
				words = appendIfNonEmpty(words, &current)
			}
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

func appendIfNonEmpty(words []string, current *strings.Builder) []string {
	if current.Len() > 0 {
		words = append(words, current.String())
		current.Reset()
	}
	return words
}

func isAllUpper(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func ToTitle(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	pascal := ExportName(s)
	runes := []rune(pascal)
	var res strings.Builder

	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			// Add underscore only if:
			// 1. Previous char was lowercase (e.g., "companyId" → "company_id")
			// 2. OR we're at the start of a new word after an acronym (e.g., "APIKey" → "api_key")
			if unicode.IsLower(prev) {
				res.WriteRune('_')
			} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				// Current is upper, prev was upper, next is lower = new word starts
				res.WriteRune('_')
			}
			// If prev and current are both upper and next is also upper (or end), keep together
			// e.g., "ID" stays "id", "URL" stays "url"
		}
		res.WriteRune(unicode.ToLower(r))
	}

	return res.String()
}

func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}
	
	// Lowercase if it's a known acronym itself
	lower := strings.ToLower(s)
	if _, ok := KnownAcronyms[lower]; ok {
		return lower
	}

	pascal := ExportName(s)
	if pascal == "" {
		return ""
	}

	runes := []rune(pascal)
	
	// If starts with acronym (like "ID" or "URL")
	// Find how many uppercase letters are at the start
	i := 0
	for i < len(runes) && unicode.IsUpper(runes[i]) {
		i++
	}

	if i > 1 && i < len(runes) && unicode.IsLower(runes[i]) {
		// "URLPath" -> i=3, runes[3]='P'. No, i=3 is 'P' which is Upper.
		// "URLpath" -> i=3, runes[3]='p' which is Lower.
		// Treat as "URL" + "path". Lowercase all but last upper: "urlPath"
		for j := 0; j < i-1; j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
	} else {
		// Just lowercase first word
		// "CompanyID" -> "companyID" (if i=1 for 'C')
		// "ID" -> "id"
		for j := 0; j < i; j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
	}

	return string(runes)
}
