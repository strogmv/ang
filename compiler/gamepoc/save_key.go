package gamepoc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The save key (owner 20.11, design/saves/compat-v1/BRIEF.md). A save depends on what the server simulates with, not
// on the menus, screens, texts and art the whole content hash covers: the key is a hash of the server's runtime package
// (world.json and the barks and wounds catalogs the server embeds besides it) without the parts that only show
// things. GamePOC.save_key classifies every top-level section of that package, so a new section cannot slip in or out
// of the key unseen:
//
//   - "checked": in the key; the saved state names it only by ids the load validates. A change loads older saves
//     after the full validation.
//   - "strict": in the key and in its strict part. The saved state refers into it by index or keeps a copy of it, so
//     no validator can tell a wrong reference from a right one; a change refuses older saves as from an older version.
//   - "out": not in the key. reads "none": the server does not decode it at all (a generated test proves it);
//     reads "aside": the server reads it, but not to simulate — to show it, pass it on or recognise older saves; the
//     reason says where.
//
// Parts inside a section can differ from their section ("out" or "strict"), by a path of "/" segments where "*" is
// every key of an object or item of a list and "**" any depth. A strict part with shape only hashes the lengths of
// what it matches (a grid's rows and columns), not the values. A strict part with each (the id field of the items its
// first "*" goes through) is hashed per item, "<path>[<id>]": a save then refuses only when an item it knows changed or
// is gone, and an item new since the save (a new resident) is no change to it (saves growth, owner 20.11).
type SaveKeyPolicy struct {
	Sections map[string]SaveKeyRule `json:"sections"`
	Parts    []SaveKeyPart          `json:"parts,omitempty"`
	// Legacy: the key a save without one (a v1 save of an earlier build in the content-hash lists) is read with —
	// the runtime package of the build that introduced the save key, pinned. Its aliases are keys in the earlier form of
	// the record (before per-item strict parts) whose strict parts are this record's: a save carrying one is read with it.
	Legacy *SaveKeyRecord `json:"legacy,omitempty"`
}

type SaveKeyRule struct {
	Key    string `json:"key"`
	Reads  string `json:"reads,omitempty"`
	Reason string `json:"reason"`
}

type SaveKeyPart struct {
	Path   string `json:"path"`
	Key    string `json:"key"`
	Reads  string `json:"reads,omitempty"`
	Shape  bool   `json:"shape,omitempty"`
	Each   string `json:"each,omitempty"`
	Reason string `json:"reason"`
}

// SaveKeyRecord is what a save file carries: the whole key and the hash of every keyed section and strict part, so a
// later build can compare whatever it holds strict then.
type SaveKeyRecord struct {
	Hash    string            `json:"hash"`
	Parts   map[string]string `json:"parts"`
	Aliases []string          `json:"aliases,omitempty"`
}

// SaveKeyCatalog is the section save_key of world.json.
type SaveKeyCatalog struct {
	Hash  string            `json:"hash"`
	Parts map[string]string `json:"parts"`
	// Strict: the sections and parts a save's record must match.
	Strict []string `json:"strict"`
	// StrictEach: the strict parts hashed per item ("<path>[<id>]" in Parts); "<path>" itself is their hash as one, for a
	// record of the earlier form.
	StrictEach []string `json:"strict_each,omitempty"`
	// Unread: what the server does not decode (out, reads none), for the generated test that proves it.
	Unread []string       `json:"unread"`
	Legacy *SaveKeyRecord `json:"legacy,omitempty"`
}

const (
	saveKeyChecked = "checked"
	saveKeyStrict  = "strict"
	saveKeyOut     = "out"
	saveReadsNone  = "none"
	saveReadsAside = "aside"
	// saveKeySection is the key's own section in world.json; it is never classified.
	saveKeySection = "save_key"
)

func saveKeyPath(path string) ([]string, error) {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		if s == "" || (i == 0 && (s == "*" || s == "**")) || (s == "**" && i == len(segs)-1) {
			return nil, fmt.Errorf("save_key part %q: empty segment, a wildcard section or a trailing **", path)
		}
	}
	return segs, nil
}

func validHexHash(h string) bool {
	raw, err := hex.DecodeString(h)
	return err == nil && len(raw) == sha256.Size && strings.ToLower(h) == h
}

func (p *SaveKeyPolicy) validate() error {
	if p == nil || len(p.Sections) == 0 {
		return fmt.Errorf("GamePOC.save_key: no sections; every section of the server's runtime package must be classified")
	}
	if _, ok := p.Sections[saveKeySection]; ok {
		return fmt.Errorf("save_key: the key's own section %q is never classified", saveKeySection)
	}
	for name, r := range p.Sections {
		if strings.TrimSpace(r.Reason) == "" {
			return fmt.Errorf("save_key section %q: no reason", name)
		}
		switch r.Key {
		case saveKeyChecked, saveKeyStrict:
			if r.Reads != "" {
				return fmt.Errorf("save_key section %q: reads is for out only", name)
			}
		case saveKeyOut:
			if r.Reads != saveReadsNone && r.Reads != saveReadsAside {
				return fmt.Errorf("save_key section %q: out needs reads none or aside", name)
			}
		default:
			return fmt.Errorf("save_key section %q: key %q is not checked, strict or out", name, r.Key)
		}
	}
	seen := map[string]bool{}
	for _, part := range p.Parts {
		segs, err := saveKeyPath(part.Path)
		if err != nil {
			return err
		}
		if seen[part.Path] {
			return fmt.Errorf("save_key part %q twice", part.Path)
		}
		seen[part.Path] = true
		if strings.TrimSpace(part.Reason) == "" {
			return fmt.Errorf("save_key part %q: no reason", part.Path)
		}
		section, ok := p.Sections[segs[0]]
		if !ok {
			return fmt.Errorf("save_key part %q: section %q is not classified", part.Path, segs[0])
		}
		switch part.Key {
		case saveKeyStrict:
			if part.Reads != "" {
				return fmt.Errorf("save_key part %q: reads is for out only", part.Path)
			}
			if section.Key != saveKeyChecked {
				return fmt.Errorf("save_key part %q: a strict part belongs in a checked section (%q is %s)", part.Path, segs[0], section.Key)
			}
			if part.Each != "" && (!containsString(segs, "*") || strings.ContainsAny(part.Each, "/*[]")) {
				return fmt.Errorf("save_key part %q: each names the id field of the items its first * goes through", part.Path)
			}
		case saveKeyOut:
			if len(segs) == 1 {
				return fmt.Errorf("save_key part %q: a whole section is left out by its section rule", part.Path)
			}
			if part.Reads != saveReadsNone && part.Reads != saveReadsAside {
				return fmt.Errorf("save_key part %q: out needs reads none or aside", part.Path)
			}
			if part.Shape || part.Each != "" {
				return fmt.Errorf("save_key part %q: shape and each are for strict parts", part.Path)
			}
			if section.Key == saveKeyOut {
				return fmt.Errorf("save_key part %q: section %q is out already", part.Path, segs[0])
			}
		default:
			return fmt.Errorf("save_key part %q: key %q is not strict or out", part.Path, part.Key)
		}
	}
	if p.Legacy != nil {
		if !validHexHash(p.Legacy.Hash) || len(p.Legacy.Parts) == 0 {
			return fmt.Errorf("save_key.legacy: a sha256 hash and the parts it was made of")
		}
		for name, h := range p.Legacy.Parts {
			if !validHexHash(h) {
				return fmt.Errorf("save_key.legacy part %q: not a sha256 hash", name)
			}
		}
		for _, h := range p.Legacy.Aliases {
			if !validHexHash(h) || h == p.Legacy.Hash {
				return fmt.Errorf("save_key.legacy alias %q: another sha256 hash of the same package", h)
			}
		}
	}
	return nil
}

// saveKeyValue decodes JSON keeping numbers as written, so the hash does not depend on float formatting.
func saveKeyValue(raw json.RawMessage) (any, error) {
	var v any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// saveKeyHash hashes the canonical JSON of v: object keys sorted (encoding/json), numbers as written.
func saveKeyHash(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// saveKeyRemove deletes what segs matches under v and returns how many places it removed.
func saveKeyRemove(v any, segs []string) int {
	if len(segs) == 0 {
		return 0
	}
	seg, rest := segs[0], segs[1:]
	n := 0
	if seg == "**" {
		n += saveKeyRemove(v, rest)
		switch x := v.(type) {
		case map[string]any:
			for _, child := range x {
				n += saveKeyRemove(child, segs)
			}
		case []any:
			for _, child := range x {
				n += saveKeyRemove(child, segs)
			}
		}
		return n
	}
	switch x := v.(type) {
	case map[string]any:
		if len(rest) == 0 {
			if seg == "*" {
				n = len(x)
				for k := range x {
					delete(x, k)
				}
				return n
			}
			if _, ok := x[seg]; ok {
				delete(x, seg)
				return 1
			}
			return 0
		}
		if seg == "*" {
			for _, child := range x {
				n += saveKeyRemove(child, rest)
			}
			return n
		}
		if child, ok := x[seg]; ok {
			return saveKeyRemove(child, rest)
		}
	case []any:
		if seg != "*" || len(rest) == 0 {
			return 0
		}
		for _, child := range x {
			n += saveKeyRemove(child, rest)
		}
	}
	return n
}

// saveKeyMatch is one place a strict part matched: its concrete path and value.
type saveKeyMatch struct {
	Path  string `json:"path"`
	Value any    `json:"value"`
}

func saveKeySelect(v any, segs []string, at string, out *[]saveKeyMatch) {
	if len(segs) == 0 {
		*out = append(*out, saveKeyMatch{Path: at, Value: v})
		return
	}
	seg, rest := segs[0], segs[1:]
	if seg == "**" {
		saveKeySelect(v, rest, at, out)
		saveKeyEach(v, at, func(child any, path string) { saveKeySelect(child, segs, path, out) })
		return
	}
	if seg == "*" {
		saveKeyEach(v, at, func(child any, path string) { saveKeySelect(child, rest, path, out) })
		return
	}
	if x, ok := v.(map[string]any); ok {
		if child, ok := x[seg]; ok {
			saveKeySelect(child, rest, at+"/"+seg, out)
		}
	}
}

// saveKeyEach visits the children of an object (keys sorted) or a list (in order).
func saveKeyEach(v any, at string, visit func(any, string)) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			visit(x[k], at+"/"+k)
		}
	case []any:
		for i, child := range x {
			visit(child, at+"/"+strconv.Itoa(i))
		}
	}
}

// saveKeyShape: the lengths of v, not its values (a list or a string by its length, an object by its keys).
func saveKeyShape(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			out[k] = saveKeyShape(child)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			out[i] = saveKeyShape(child)
		}
		return out
	case string:
		return len([]rune(x))
	case nil:
		return "null"
	case bool:
		return "bool"
	default:
		return "number"
	}
}

// computeSaveKey classifies and hashes the server's runtime package (top-level sections by name, raw JSON).
func computeSaveKey(p *SaveKeyPolicy, sections map[string]json.RawMessage) (SaveKeyCatalog, error) {
	var out SaveKeyCatalog
	if err := p.validate(); err != nil {
		return out, err
	}
	names := make([]string, 0, len(sections))
	for name := range sections {
		if name == saveKeySection {
			continue
		}
		if _, ok := p.Sections[name]; !ok {
			return out, fmt.Errorf("world.json section %q is in neither the save key nor its leave-out list: classify it in GamePOC.save_key.sections (server/cue/poc) with a reason", name)
		}
		names = append(names, name)
	}
	for name := range p.Sections {
		if _, ok := sections[name]; !ok {
			return out, fmt.Errorf("save_key section %q is not in the server's runtime package any more: remove it from GamePOC.save_key.sections", name)
		}
	}
	sort.Strings(names)
	values := map[string]any{}
	for _, name := range names {
		if p.Sections[name].Key == saveKeyOut {
			continue
		}
		v, err := saveKeyValue(sections[name])
		if err != nil {
			return out, fmt.Errorf("save key: section %s: %w", name, err)
		}
		values[name] = v
	}
	out.Parts = map[string]string{}
	for _, part := range p.Parts {
		if part.Key != saveKeyOut {
			continue
		}
		segs, _ := saveKeyPath(part.Path)
		if saveKeyRemove(values[segs[0]], segs[1:]) == 0 {
			return out, fmt.Errorf("save_key part %q matches nothing in the runtime package: correct or remove it", part.Path)
		}
		if part.Reads == saveReadsNone {
			out.Unread = append(out.Unread, part.Path)
		}
	}
	for _, name := range names {
		rule := p.Sections[name]
		if rule.Key == saveKeyOut {
			if rule.Reads == saveReadsNone {
				out.Unread = append(out.Unread, name)
			}
			continue
		}
		h, err := saveKeyHash(values[name])
		if err != nil {
			return out, err
		}
		out.Parts[name] = h
		if rule.Key == saveKeyStrict {
			out.Strict = append(out.Strict, name)
		}
	}
	for _, part := range p.Parts {
		if part.Key != saveKeyStrict {
			continue
		}
		segs, _ := saveKeyPath(part.Path)
		var matches []saveKeyMatch
		saveKeySelect(values[segs[0]], segs[1:], segs[0], &matches)
		if len(matches) == 0 {
			return out, fmt.Errorf("save_key part %q matches nothing in the runtime package: correct or remove it", part.Path)
		}
		if part.Shape {
			for i := range matches {
				matches[i].Value = saveKeyShape(matches[i].Value)
			}
		}
		h, err := saveKeyHash(matches)
		if err != nil {
			return out, err
		}
		out.Parts[part.Path] = h
		if part.Each == "" {
			out.Strict = append(out.Strict, part.Path)
			continue
		}
		items, err := saveKeyEachItem(values[segs[0]], segs, part)
		if err != nil {
			return out, err
		}
		for id, item := range items {
			if part.Shape {
				for i := range item {
					item[i].Value = saveKeyShape(item[i].Value)
				}
			}
			if out.Parts[part.Path+"["+id+"]"], err = saveKeyHash(item); err != nil {
				return out, err
			}
		}
		out.StrictEach = append(out.StrictEach, part.Path)
	}
	sort.Strings(out.Strict)
	sort.Strings(out.StrictEach)
	sort.Strings(out.Unread)
	out.Hash = saveKeyRecordHash(out.Parts)
	if p.Legacy != nil {
		legacy := *p.Legacy
		out.Legacy = &legacy
	}
	return out, nil
}

// saveKeyEachItem: the matches of a per-item strict part grouped by the id of the item its first "*" goes through.
func saveKeyEachItem(section any, segs []string, part SaveKeyPart) (map[string][]saveKeyMatch, error) {
	first := 0
	for i, s := range segs {
		if s == "*" {
			first = i
			break
		}
	}
	var containers []saveKeyMatch
	saveKeySelect(section, segs[1:first], segs[0], &containers)
	out := map[string][]saveKeyMatch{}
	for _, c := range containers {
		var problem error
		saveKeyEach(c.Value, c.Path, func(item any, at string) {
			fields, ok := item.(map[string]any)
			id, named := fields[part.Each].(string)
			if !ok || !named || id == "" || strings.ContainsAny(id, "[]") {
				problem = fmt.Errorf("save_key part %q: %s has no %s to name it by", part.Path, at, part.Each)
				return
			}
			if _, twice := out[id]; twice {
				problem = fmt.Errorf("save_key part %q: %s %q twice", part.Path, part.Each, id)
				return
			}
			// paths inside the item, not through its place in the list: one more item before it changes nothing of it
			var matches []saveKeyMatch
			saveKeySelect(item, segs[first+1:], "", &matches)
			out[id] = matches
		})
		if problem != nil {
			return nil, problem
		}
	}
	return out, nil
}

// saveKeyRecordHash: the whole key from the hashes of its parts.
func saveKeyRecordHash(parts map[string]string) string {
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(parts[name]))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// withSaveKey appends the section save_key to the indented world.json of json.MarshalIndent(compiled, "", "  ").
func withSaveKey(world []byte, key SaveKeyCatalog) ([]byte, error) {
	body, ok := bytes.CutSuffix(world, []byte("\n}"))
	if !ok || len(body) < 2 {
		return nil, fmt.Errorf("world.json is not an indented object")
	}
	section, err := json.MarshalIndent(key, "  ", "  ")
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(world)+len(section)+32)
	out = append(out, body...)
	out = append(out, ",\n  \""+saveKeySection+"\": "...)
	out = append(out, section...)
	out = append(out, "\n}"...)
	return out, nil
}
