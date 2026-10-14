package compiler

import (
	"regexp"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/pkg/names"
)

// typeObjectsTheCodeReachesInto gives an inline object of an operation's
// input or output its named Go struct (<Op>Response<Field>, the name its
// TypeScript, Zod and OpenAPI shape already has) when the operation's own
// code reaches into it: resp.Interest.Signs30d, page.Settings.Remind, or the
// type by name (port.GetXResponseInterest{...}). Such code cannot compile
// against the loose `any` an object gets otherwise.
//
// Every other inline object stays loose: flows fill those from map literals,
// domain JSON and structurally equal domain structs (typing all of them broke
// ~65 flows of dealingi-back). A list of objects is a struct already; objects
// inside it follow the same rule. A promoted object's own objects do too, and
// an object reached into makes every object around it a struct.
func typeObjectsTheCodeReachesInto(services []normalizer.Service) {
	for si := range services {
		for mi := range services[si].Methods {
			method := &services[si].Methods[mi]
			code := methodCodeText(*method)
			if strings.TrimSpace(code) == "" {
				continue
			}
			typeReachedObjects(method.Input.Fields, code)
			typeReachedObjects(method.Output.Fields, code)
		}
	}
}

// typeReachedObjects promotes the reached loose objects among fields (and
// below them); it reports whether it promoted any.
func typeReachedObjects(fields []normalizer.Field, code string) bool {
	promoted := false
	for i := range fields {
		f := &fields[i]
		if len(f.ItemFields) == 0 || f.ItemTypeName == "" {
			continue
		}
		switch {
		case isLooseObject(*f):
			inner := typeReachedObjects(f.ItemFields, code)
			if inner || codeReachesInto(code, *f) {
				f.Type = f.ItemTypeName
				delete(f.Metadata, "shape")
				promoted = true
			}
		case strings.Contains(f.Type, "[]"+f.ItemTypeName):
			// A list of objects: its item is a struct already.
			if typeReachedObjects(f.ItemFields, code) {
				promoted = true
			}
		}
	}
	return promoted
}

// isLooseObject reports an inline object kept as any in Go (not a map, whose
// shape is map[string]T).
func isLooseObject(f normalizer.Field) bool {
	shape, _ := f.Metadata["shape"].(string)
	return shape != "" && shape == f.ItemTypeName
}

// codeReachesInto reports code that selects a field of the object
// (.Interest.Signs30d) or names its type.
func codeReachesInto(code string, f normalizer.Field) bool {
	if regexp.MustCompile(`\b` + regexp.QuoteMeta(f.ItemTypeName) + `\b`).MatchString(code) {
		return true
	}
	inner := make([]string, 0, len(f.ItemFields))
	for _, child := range f.ItemFields {
		inner = append(inner, regexp.QuoteMeta(names.ToGoName(child.Name)))
	}
	pattern := `\.` + regexp.QuoteMeta(names.ToGoName(f.Name)) + `\s*\.\s*(` + strings.Join(inner, "|") + `)\b`
	return regexp.MustCompile(pattern).MatchString(code)
}
