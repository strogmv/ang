package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

// FieldMetaQuery is the field metadata key of a typed query parameter
// (@query on an operation input field).
const FieldMetaQuery = "query"

// Typed query parameter kinds (@query(type=...)).
const (
	QueryTypeString  = "string"
	QueryTypeInt     = "int"
	QueryTypeInt64   = "int64"
	QueryTypeFloat   = "float"
	QueryTypeBool    = "bool"
	QueryTypeDate    = "date"
	QueryTypeTime    = "time"
	QueryTypeStrings = "[]string"
)

// QueryParamSpec is a typed query parameter:
//
//	from:   string   @query(type="date")                    // ?from=2026-10-14 -> time.Time
//	at:     string   @query(type="time", format="2006-01-02 15:04")
//	limit:  int      @query(required=true)
//	ids:    [...string] @query()                            // ?ids=a&ids=b or ?ids=a,b
//	tags:   [...string] @query(format="csv", name="tag")
//
// The field gets the Go type of its kind (time.Time for date and time). A
// value that does not parse is a 400 problem+json answer naming the
// parameter; required=true makes a missing one a 400 too.
type QueryParamSpec struct {
	Type     string
	Format   string // time layout for date/time; "csv", "multi" or "" (both) for []string
	Name     string // query key; "" means the field's JSON name
	Required bool
}

// queryGoTypes is the Go type of each kind.
var queryGoTypes = map[string]string{
	QueryTypeString:  "string",
	QueryTypeInt:     "int",
	QueryTypeInt64:   "int64",
	QueryTypeFloat:   "float64",
	QueryTypeBool:    "bool",
	QueryTypeDate:    "time.Time",
	QueryTypeTime:    "time.Time",
	QueryTypeStrings: "[]string",
}

// parseQueryAttr reads @query on an operation input field and retypes the
// field. ok is false when the field has no @query.
func parseQueryAttr(v cue.Value, field *Field) (bool, error) {
	attr := v.Attribute("query")
	if attr.Err() != nil {
		return false, nil
	}
	spec := QueryParamSpec{}
	if s, found, _ := attr.Lookup(0, "type"); found {
		spec.Type = strings.TrimSpace(s)
	}
	if s, found, _ := attr.Lookup(0, "format"); found {
		spec.Format = strings.TrimSpace(s)
	}
	if s, found, _ := attr.Lookup(0, "name"); found {
		spec.Name = strings.TrimSpace(s)
	}
	if s, found, _ := attr.Lookup(0, "required"); found {
		switch strings.TrimSpace(s) {
		case "true":
			spec.Required = true
		case "false", "":
		default:
			return true, fmt.Errorf("field %s: @query required must be true or false, got %q", field.Name, s)
		}
	}
	if spec.Type == "" {
		switch field.Type {
		case "int":
			spec.Type = QueryTypeInt
		case "int64":
			spec.Type = QueryTypeInt64
		case "float64":
			spec.Type = QueryTypeFloat
		case "bool":
			spec.Type = QueryTypeBool
		case "time.Time":
			spec.Type = QueryTypeTime
		case "[]string":
			spec.Type = QueryTypeStrings
		default:
			spec.Type = QueryTypeString
		}
	}
	goType, known := queryGoTypes[spec.Type]
	if !known {
		return true, fmt.Errorf("field %s: @query type %q: want string, int, int64, float, bool, date, time or []string", field.Name, spec.Type)
	}
	switch spec.Type {
	case QueryTypeDate:
		if spec.Format == "" {
			spec.Format = "2006-01-02"
		}
	case QueryTypeTime:
		if spec.Format == "" {
			spec.Format = "2006-01-02T15:04:05Z07:00"
		}
	case QueryTypeStrings:
		if spec.Format != "" && spec.Format != "csv" && spec.Format != "multi" {
			return true, fmt.Errorf("field %s: @query format for []string is \"csv\" or \"multi\", got %q", field.Name, spec.Format)
		}
	default:
		if spec.Format != "" {
			return true, fmt.Errorf("field %s: @query format applies to date, time and []string, not %s", field.Name, spec.Type)
		}
	}
	field.Type = goType
	field.IsList = spec.Type == QueryTypeStrings
	if field.Metadata == nil {
		field.Metadata = map[string]any{}
	}
	field.Metadata[FieldMetaQuery] = map[string]any{
		"type": spec.Type, "format": spec.Format, "name": spec.Name, "required": spec.Required,
	}
	return true, nil
}

// FieldQueryParam returns the typed query parameter a field declares, nil
// for a field without @query. It reads the metadata after an IR round trip.
func FieldQueryParam(meta map[string]any) *QueryParamSpec {
	raw, ok := meta[FieldMetaQuery].(map[string]any)
	if !ok {
		return nil
	}
	spec := &QueryParamSpec{}
	spec.Type, _ = raw["type"].(string)
	spec.Format, _ = raw["format"].(string)
	spec.Name, _ = raw["name"].(string)
	spec.Required, _ = raw["required"].(bool)
	if spec.Type == "" {
		return nil
	}
	return spec
}
