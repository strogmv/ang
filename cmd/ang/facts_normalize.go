package main

import "strings"

func canonicalFactFieldName(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	key = strings.NewReplacer("_", "", "-", "", " ", "", ".", "").Replace(key)
	switch key {
	case "avatar", "avatarurl", "photo", "photourl", "image", "imageurl", "picture", "pictureurl",
		"profilepicture", "profilepictureurl", "profilephoto", "profilephotourl", "profileimage", "profileimageurl":
		return "PhotoURL"
	default:
		return strings.TrimSpace(name)
	}
}

func canonicalizeFactField(f FactField) FactField {
	f.Name = canonicalFactFieldName(f.Name)
	return f
}

func canonicalizeFactFields(fields []FactField) []FactField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]FactField, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		field = canonicalizeFactField(field)
		key := normalizeFactID(field.Name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, field)
	}
	return out
}

func canonicalizeFactsEnvelopeFields(env *FactsEnvelope) {
	for i := range env.Entities {
		env.Entities[i].Fields = canonicalizeFactFields(env.Entities[i].Fields)
	}
	for i := range env.Operations {
		env.Operations[i].InputFields = canonicalizeFactFields(env.Operations[i].InputFields)
		env.Operations[i].OutputFields = canonicalizeFactFields(env.Operations[i].OutputFields)
	}
	for i := range env.Events {
		env.Events[i].PayloadFields = canonicalizeFactFields(env.Events[i].PayloadFields)
	}
}
