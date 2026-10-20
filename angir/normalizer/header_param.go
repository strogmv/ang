package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

// FieldMetaHeader is the field metadata key of an input read from a request
// header (@header on an operation input field).
const FieldMetaHeader = "header"

// parseHeaderAttr reads @header on an operation input field:
//
//	secretToken: string @header(name="X-Telegram-Bot-Api-Secret-Token")
//	secretToken: string @header("X-Telegram-Bot-Api-Secret-Token")
//
// The handler sets the field from that header, always (an absent header is
// ""), so a value in the body or the query can never stand in for it. Only a
// string field can come from a header.
func parseHeaderAttr(v cue.Value, field *Field) error {
	attr := v.Attribute("header")
	if attr.Err() != nil {
		return nil
	}
	name := ""
	if s, found, _ := attr.Lookup(0, "name"); found {
		name = strings.TrimSpace(s)
	} else if attr.NumArgs() > 0 {
		if s, err := attr.String(0); err == nil && !strings.Contains(s, "=") {
			name = strings.TrimSpace(s)
		}
	}
	if name == "" {
		return fmt.Errorf("field %s: @header needs the header's name, e.g. @header(name=\"X-Api-Key\")", field.Name)
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c)) {
			return fmt.Errorf("field %s: @header name %q is not a header name", field.Name, name)
		}
	}
	if field.Type != "string" || field.IsList {
		return fmt.Errorf("field %s: @header fields are strings, got %s", field.Name, field.Type)
	}
	if field.Metadata != nil && field.Metadata[FieldMetaQuery] != nil {
		return fmt.Errorf("field %s: @header and @query exclude each other", field.Name)
	}
	if field.Metadata == nil {
		field.Metadata = map[string]any{}
	}
	field.Metadata[FieldMetaHeader] = name
	return nil
}

// FieldHeaderName is the request header a field is read from, "" for a field
// without @header. It reads the metadata after an IR round trip.
func FieldHeaderName(meta map[string]any) string {
	s, _ := meta[FieldMetaHeader].(string)
	return s
}
