package normalizer

import (
	"fmt"

	"cuelang.org/go/cue"
)

// FieldMetaNullable marks an optional scalar input/output field that has
// three states — absent (or null), and each value — written `x?: bool
// @nullable()`. The port struct gets a pointer (*bool), the TypeScript type
// `x?: T | null`, OpenAPI `nullable: true`; Field.Type stays the scalar.
const FieldMetaNullable = "nullable"

var nullableTypes = map[string]bool{
	"bool": true, "int": true, "int64": true, "float64": true, "string": true, "time.Time": true,
}

// applyNullable reads @nullable() of a field.
func applyNullable(field *Field, v cue.Value) error {
	if attr := v.Attribute("nullable"); attr.Err() != nil {
		return nil
	}
	if !field.IsOptional {
		return fmt.Errorf("%s: field %s is @nullable but not optional: write %s?: %s @nullable()", field.Source, field.Name, field.Name, field.Type)
	}
	if !nullableTypes[field.Type] {
		return fmt.Errorf("%s: field %s is @nullable but its type is %s; @nullable is for bool, int, int64, float, string and time fields", field.Source, field.Name, field.Type)
	}
	if field.Metadata == nil {
		field.Metadata = map[string]any{}
	}
	field.Metadata[FieldMetaNullable] = true
	return nil
}

// IsNullable reports whether f is a three-state field (@nullable()).
func IsNullable(f Field) bool {
	v, _ := f.Metadata[FieldMetaNullable].(bool)
	return v
}

// rejectNullableFields fails on @nullable in an entity: only an operation's
// port types are generated with pointers.
func rejectNullableFields(e Entity) error {
	var walk func([]Field) error
	walk = func(fields []Field) error {
		for _, f := range fields {
			if IsNullable(f) {
				return fmt.Errorf("%s: field %s.%s is @nullable, which only operation inputs and outputs support", f.Source, e.Name, f.Name)
			}
			if err := walk(f.ItemFields); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(e.Fields)
}
