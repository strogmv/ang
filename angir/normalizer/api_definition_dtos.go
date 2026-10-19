package normalizer

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"cuelang.org/go/cue"
)

var domainTypeRef = regexp.MustCompile(`\bdomain\.([A-Za-z_][A-Za-z0-9_]*)`)

// ExtractAPIDefinitionDTOs registers the cue/api definitions operations use
// as field types (`lines: [...#QuoteLine]`, `buyer: #Party`) as DTO entities.
// The type reader writes any `#X` as domain.X, so without this an api
// definition was "unknown entity type X" and had to be moved to cue/domain
// with @dto(only="true"). Only referenced definitions are registered (and
// those they reference); a cue/domain entity of the same name wins. The
// returned entities are the new ones, sorted by name.
func (n *Normalizer) ExtractAPIDefinitionDTOs(api cue.Value, services []Service, entities []Entity) ([]Entity, error) {
	if !api.Exists() {
		return nil, nil
	}
	known := make(map[string]bool, len(entities))
	for _, e := range entities {
		known[e.Name] = true
	}
	var queue []string
	queued := map[string]bool{}
	note := func(fields []Field) {
		var walk func([]Field)
		walk = func(fields []Field) {
			for _, f := range fields {
				candidates := []string{f.ItemTypeName}
				for _, m := range domainTypeRef.FindAllStringSubmatch(f.Type, -1) {
					candidates = append(candidates, m[1])
				}
				for _, name := range candidates {
					if name != "" && !known[name] && !queued[name] {
						queued[name] = true
						queue = append(queue, name)
					}
				}
				walk(f.ItemFields)
			}
		}
		walk(fields)
	}
	for _, svc := range services {
		for _, m := range svc.Methods {
			note(m.Input.Fields)
			note(m.Output.Fields)
		}
	}
	var added []Entity
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		def, ok := apiDefinition(api, name)
		if !ok {
			continue // not an api definition: the IR check reports the unknown type
		}
		ent, err := n.parseEntity(name, def)
		if err != nil {
			return nil, err
		}
		if ent.Metadata == nil {
			ent.Metadata = map[string]any{}
		}
		ent.Metadata["dto"] = true
		ent.Metadata["api_definition"] = true
		known[name] = true
		added = append(added, ent)
		note(ent.Fields)
	}
	sort.Slice(added, func(i, j int) bool { return added[i].Name < added[j].Name })
	return added, nil
}

// apiDefinition finds #Name (or #name, which the type reader exports to Name)
// among the api package's definitions; it must be a struct.
func apiDefinition(api cue.Value, name string) (cue.Value, bool) {
	tries := []string{"#" + name}
	if r := []rune(name); len(r) > 0 {
		r[0] = unicode.ToLower(r[0])
		tries = append(tries, "#"+string(r))
	}
	for _, label := range tries {
		v := api.LookupPath(cue.MakePath(cue.Def(label)))
		if v.Exists() && v.IncompleteKind() == cue.StructKind && !strings.Contains(label, ".") {
			return v, true
		}
	}
	return cue.Value{}, false
}
