package emitter

import (
	"strings"
	"unicode"

	"github.com/strogmv/ang/compiler/pkg/names"
)

// knownAcronyms are common abbreviations that should stay uppercase
var knownAcronyms = map[string]string{
	"id": "ID", "api": "API", "jwt": "JWT", "ttl": "TTL",
	"url": "URL", "http": "HTTP", "https": "HTTPS", "sql": "SQL",
	"json": "JSON", "xml": "XML", "html": "HTML", "css": "CSS",
	"uuid": "UUID", "uri": "URI", "ip": "IP", "tcp": "TCP", "udp": "UDP",
	"fsm": "FSM", "dto": "DTO", "dao": "DAO", "orm": "ORM",
	"ai": "AI", "ml": "ML", "ui": "UI", "io": "IO", "s3": "S3",
	"vat": "VAT", "sku": "SKU", "db": "DB", "rpc": "RPC",
}

// knownCompounds are common compound words that should be split
var knownCompounds = map[string]string{
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
// Handles various input formats: camelCase, PascalCase, snake_case, SCREAMING_CASE, mixed.
func ExportName(name string) string {
	if name == "" {
		return ""
	}

	// Check for known compound words first
	lower := strings.ToLower(name)
	if compound, ok := knownCompounds[lower]; ok {
		return compound
	}

	// Split by underscores, dots, dashes, or case changes
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

		// Split on CamelCase: "userId" -> "user", "Id"
		if i > 0 && unicode.IsUpper(r) && unicode.IsLower(runes[i-1]) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}

		// Split on transition from uppercase sequence to lowercase: "APIKey" -> "API", "Key"
		if i > 1 && unicode.IsUpper(runes[i-1]) && unicode.IsLower(r) && unicode.IsUpper(runes[i-2]) {
			// Move last uppercase char to new word
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
		if acronym, ok := knownAcronyms[lower]; ok {
			words[i] = acronym
		} else {
			// Title case
			r := []rune(lower)
			r[0] = unicode.ToUpper(r[0])
			words[i] = string(r)
		}
	}

	return strings.Join(words, "")
}

func JSONName(name string) string {
	return names.ToJSONName(name)
}

func DBName(name string) string {
	return strings.ToLower(ExportName(name))
}

func ToTitle(s string) string {

	if s == "" { return "" }

	r := []rune(s)

	r[0] = unicode.ToUpper(r[0])

	return string(r)

}



func ToSnakeCase(s string) string {

	if s == "" {

		return ""

	}

	var res strings.Builder

	for i, r := range s {

		if i > 0 && unicode.IsUpper(r) {

			res.WriteRune('_')

		}

		res.WriteRune(unicode.ToLower(r))

	}

	return res.String()

}
