package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

// Nested shapes of an operation's input and output.
//
// A field written inline in CUE gets its own named type at any depth:
//
//	output: {
//	    settings: {remind: bool}                    // shape GetXResponseSettings
//	    pages: [...{                                // []GetXResponsePagesItem
//	        steps: [...{anchor: string}]            // []GetXResponsePagesItemStepsItem
//	        labels: [string]: string                // shape map[string]string
//	    }]
//	}
//
// detectType alone cannot see these shapes: it answers map[string]any for an
// object and, because the printed list mentions "string", []string for a list
// of objects. A shape that still has no type here is a build error that names
// the CUE field; it is never left to degrade silently.
//
// Lists are typed in Go too: their old type ([]string) was wrong. An object
// or a map keeps its Go type map[string]any (the flows fill these from loose
// domain JSON and map literals), and carries its exact shape in
// Metadata["shape"] for TypeScript, Zod and OpenAPI; see FieldShape.

// FieldShape answers the exact type of a field for the API description and
// the SDK: the shape of an inline object or map, else its Go type.
func FieldShape(f Field) string {
	if shape, ok := f.Metadata["shape"].(string); ok && shape != "" {
		return shape
	}
	return f.Type
}

// setShape keeps the Go type of an object or map field loose and records its shape.
func setShape(field *Field, shape string) {
	field.Type = "map[string]any"
	if field.Metadata == nil {
		field.Metadata = map[string]any{}
	}
	field.Metadata["shape"] = shape
}

// CollectGoNestedTypes is CollectNestedTypes limited to the types a Go
// declaration uses: the item types of lists, not the shapes of loose objects.
func CollectGoNestedTypes(fields []Field) []Entity {
	seen := map[string]struct{}{}
	var out []Entity
	var walk func([]Field)
	walk = func(fs []Field) {
		for _, f := range fs {
			if f.ItemTypeName == "" || len(f.ItemFields) == 0 || !strings.Contains(f.Type, f.ItemTypeName) {
				continue
			}
			if _, ok := seen[f.ItemTypeName]; ok {
				continue
			}
			seen[f.ItemTypeName] = struct{}{}
			out = append(out, Entity{Name: f.ItemTypeName, Fields: f.ItemFields})
			walk(f.ItemFields)
		}
	}
	walk(fields)
	return out
}

// nestedTypeOwner is the prefix of the type names made for fields of owner.
type nestedTypeOwner struct {
	name string
	// dto is true for operation input/output (…Request, …Response). Only
	// those get inline object and map types: domain entities, events and
	// config keep their storage types and only reject what they cannot hold.
	dto bool
	// top is true for the fields of the operation input/output itself, where
	// a list named data keeps its historical type name <Op>Data.
	top bool
	// inlineRefs is true inside the value of a map: there a CUE definition
	// (#Column) gets an inline type too. Elsewhere detectType keeps its
	// historical domain.X for #X, which existing operations rely on.
	inlineRefs bool
}

// localDefinition reports a value written as a CUE definition (#X).
func localDefinition(v cue.Value) bool {
	_, path := v.ReferencePath()
	sel := path.Selectors()
	return len(sel) > 0 && strings.HasPrefix(sel[len(sel)-1].String(), "#")
}

func isDTOName(name string) bool {
	return strings.HasSuffix(name, "Request") || strings.HasSuffix(name, "Response")
}

// typeNestedField gives field (already typed by detectType) its nested type.
func (n *Normalizer) typeNestedField(owner nestedTypeOwner, label string, val cue.Value, field *Field) error {
	switch val.IncompleteKind() {
	case cue.ListKind:
		return n.typeNestedList(owner, label, val, field)
	case cue.StructKind:
		if !owner.dto {
			return nil
		}
		if field.Type == "map[string]any" || (owner.inlineRefs && localDefinition(val)) {
			return n.typeNestedObject(owner, label, val, field)
		}
	}
	return nil
}

func (n *Normalizer) typeNestedList(owner nestedTypeOwner, label string, val cue.Value, field *Field) error {
	field.IsList = true
	elem := val.LookupPath(cue.MakePath(cue.AnyIndex))
	inlineRef := owner.dto && owner.inlineRefs && elem.Exists() && localDefinition(elem)
	if strings.HasPrefix(field.Type, "[]domain.") && !inlineRef {
		field.ItemTypeName = strings.TrimPrefix(field.Type, "[]domain.")
		return nil
	}
	if !elem.Exists() {
		return nil
	}
	itemName := owner.name + exportName(label) + "Item"
	if owner.top && strings.EqualFold(label, "data") {
		itemName = owner.name + "Data"
	}
	switch {
	case !owner.dto:
		if elem.IncompleteKind() != cue.StructKind {
			return nil
		}
	case elem.IncompleteKind() == cue.ListKind || (elem.IncompleteKind() == cue.StructKind && hasOnlyPattern(elem)):
		// A list of lists or of maps: typed like a map value.
		goType, named, fields, err := n.mapValueType(owner, itemName, label, elem)
		if err != nil {
			return err
		}
		field.Type = "[]" + goType
		if named != "" {
			field.ItemTypeName = named
			field.ItemFields = fields
		}
		return nil
	case elem.IncompleteKind() != cue.StructKind:
		return nil
	}
	if _, path := elem.ReferencePath(); len(path.Selectors()) > 0 && !inlineRef {
		// A list of a named definition (#Entity, domain.X) is typed by detectType.
		return nil
	}
	itemFields, err := n.parseInlineFields(nestedTypeOwner{name: itemName, dto: owner.dto, inlineRefs: owner.inlineRefs}, elem)
	if err != nil {
		return err
	}
	if len(itemFields) == 0 {
		if hasPatternFields(elem) {
			return fmt.Errorf("%s: field %q is a list of maps, which ANG cannot type; give the item named fields", formatPos(val), label)
		}
		return nil
	}
	field.Type = "[]" + itemName
	field.ItemTypeName = itemName
	field.ItemFields = itemFields
	return nil
}

func (n *Normalizer) typeNestedObject(owner nestedTypeOwner, label string, val cue.Value, field *Field) error {
	if _, path := val.ReferencePath(); len(path.Selectors()) > 0 && !(owner.inlineRefs && localDefinition(val)) {
		return nil
	}
	typeName := owner.name + exportName(label)
	fields, err := n.parseInlineFields(nestedTypeOwner{name: typeName, dto: owner.dto, inlineRefs: owner.inlineRefs}, val)
	if err != nil {
		return err
	}
	if len(fields) > 0 {
		setShape(field, typeName)
		field.ItemTypeName = typeName
		field.ItemFields = fields
		return nil
	}
	// No named fields: a map ([string]: T) or an open object ({...}).
	elem := val.LookupPath(cue.MakePath(cue.AnyString))
	if !elem.Exists() || isTopValue(elem) {
		// {...}: an open object of anything stays loose everywhere.
		return nil
	}
	goType, named, namedFields, err := n.mapValueType(owner, typeName+"Value", label, elem)
	if err != nil {
		return err
	}
	setShape(field, "map[string]"+goType)
	if named != "" {
		field.ItemTypeName = named
		field.ItemFields = namedFields
	}
	return nil
}

// mapValueType types the value of a map: a scalar, a list, an object (named
// name) or another map. The chain ends in at most one named type, returned
// with its fields.
func (n *Normalizer) mapValueType(owner nestedTypeOwner, name, label string, v cue.Value) (string, string, []Field, error) {
	switch v.IncompleteKind() {
	case cue.StringKind:
		return "string", "", nil, nil
	case cue.IntKind:
		return "int", "", nil, nil
	case cue.FloatKind, cue.NumberKind:
		return "float64", "", nil, nil
	case cue.BoolKind:
		return "bool", "", nil, nil
	case cue.ListKind:
		elem := v.LookupPath(cue.MakePath(cue.AnyIndex))
		if !elem.Exists() {
			return "[]any", "", nil, nil
		}
		if elem.IncompleteKind() == cue.StructKind && !hasOnlyPattern(elem) {
			itemName := name + "Item"
			fields, err := n.parseInlineFields(nestedTypeOwner{name: itemName, dto: owner.dto, inlineRefs: true}, elem)
			if err != nil {
				return "", "", nil, err
			}
			return "[]" + itemName, itemName, fields, nil
		}
		inner, named, fields, err := n.mapValueType(owner, name+"Item", label, elem)
		return "[]" + inner, named, fields, err
	case cue.StructKind:
		if hasOnlyPattern(v) {
			inner, named, fields, err := n.mapValueType(owner, name+"Value", label, v.LookupPath(cue.MakePath(cue.AnyString)))
			return "map[string]" + inner, named, fields, err
		}
		fields, err := n.parseInlineFields(nestedTypeOwner{name: name, dto: owner.dto, inlineRefs: true}, v)
		if err != nil {
			return "", "", nil, err
		}
		if len(fields) == 0 {
			return "map[string]any", "", nil, nil
		}
		return name, name, fields, nil
	case cue.TopKind:
		return "any", "", nil, nil
	}
	return "", "", nil, fmt.Errorf("%s: field %q has a map value ANG cannot type (%v)", formatPos(v), label, v.IncompleteKind())
}

// hasOnlyPattern reports a struct with no named fields and a [string]: T constraint.
func hasOnlyPattern(v cue.Value) bool {
	if !hasPatternFields(v) {
		return false
	}
	iter, err := v.Fields(cue.Optional(true))
	if err != nil {
		return false
	}
	return !iter.Next()
}

// isTopValue reports a value that accepts anything (`_`): it stays any.
func isTopValue(v cue.Value) bool {
	return v.IncompleteKind() == cue.TopKind
}

// hasPatternFields reports a struct constrained only by [string]: T.
func hasPatternFields(v cue.Value) bool {
	return v.LookupPath(cue.MakePath(cue.AnyString)).Exists()
}

// collectNestedTypes walks fields and returns every nested type they use,
// depth first, each name once. Emitters declare these next to the
// operation's own input and output types.
func CollectNestedTypes(fields []Field) []Entity {
	seen := map[string]struct{}{}
	var out []Entity
	var walk func([]Field)
	walk = func(fs []Field) {
		for _, f := range fs {
			if f.ItemTypeName == "" || len(f.ItemFields) == 0 {
				continue
			}
			if _, ok := seen[f.ItemTypeName]; ok {
				continue
			}
			seen[f.ItemTypeName] = struct{}{}
			out = append(out, Entity{Name: f.ItemTypeName, Fields: f.ItemFields})
			walk(f.ItemFields)
		}
	}
	walk(fields)
	return out
}
