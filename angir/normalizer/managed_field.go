package normalizer

// FieldMetaManaged is the field metadata key of @managed() on an entity
// field: a service field (console roles, a status, a plan) that only its own
// operations write. A generic mapping.Map of a request body onto a loaded
// row (AdminUpdateX) keeps the row's value, as it does for @secret fields.
const FieldMetaManaged = "managed"

// FieldIsManaged reports @managed() (after an IR round trip too).
func FieldIsManaged(meta map[string]any) bool {
	b, _ := meta[FieldMetaManaged].(bool)
	return b
}

// BodyProtectedFields are the fields of an entity a request body must not
// write onto a loaded row: @secret (and @gdpr("sensitive")) ones, which the
// body never carries, and @managed() ones. CUE names, in entity order.
func BodyProtectedFields(ent Entity) []string {
	var out []string
	for _, f := range ent.Fields {
		if f.IsSecret || FieldIsManaged(f.Metadata) {
			out = append(out, f.Name)
		}
	}
	return out
}
