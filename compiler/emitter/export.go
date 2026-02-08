package emitter

import (
	"strings"
	"unicode"

	"github.com/strogmv/ang/compiler/pkg/names"
)

// ExportName converts internal names to Go-exported names (PascalCase with ID normalization).
// Handles various input formats: camelCase, PascalCase, snake_case, SCREAMING_CASE, mixed.
func ExportName(name string) string {
	return names.ToGoName(name)
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
