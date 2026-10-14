package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/token"
)

const (
	// CodeDTOMarkerDeprecated: a `_dto: true` field. The DTO marker is the
	// attribute @dto(only="true").
	CodeDTOMarkerDeprecated = "DTO_MARKER_DEPRECATED"
	// CodeEntityWithoutID: a struct entity with no id field and no DTO marker.
	CodeEntityWithoutID = "ENTITY_WITHOUT_ID"
)

// IsDTO reports whether an entity carries the DTO marker @dto(only="true"):
// a shape of an answer or a projection that nothing stores, so it gets no
// table and no repository.
func IsDTO(e Entity) bool {
	dto, _ := e.Metadata["dto"].(bool)
	return dto
}

// LintEntityMarkers checks how the entities of cue/domain are marked (item 36
// of docs/tech-debt-backlog.md). val is the domain value ExtractEntities read
// the entities from.
//
// The DTO marker is the attribute @dto(only="true") on the entity. An
// attribute stays where it is written; it is not inherited by an entity that
// embeds or unifies with the marked one. A hidden `_dto: true` field would
// be, which once nearly turned a real entity into a DTO, and ANG does not
// read it (CUE paths cannot name hidden fields), so it marks nothing. Each
// `_dto` field is reported once, where it is written: DTO_MARKER_DEPRECATED,
// fixed by `ang fix dto`.
//
// An entity stored in PostgreSQL without an id field and without the marker
// gets no table (the SQL emitter skips it): ENTITY_WITHOUT_ID, so that is a
// decision and not an accident. Enum and scalar declarations are not struct
// entities and are not linted.
func (n *Normalizer) LintEntityMarkers(val cue.Value, entities []Entity) {
	if !val.Exists() {
		return
	}
	byName := make(map[string]Entity, len(entities))
	for _, e := range entities {
		byName[e.Name] = e
	}
	iter, err := val.Fields(cue.All())
	if err != nil {
		return
	}
	reported := map[string]bool{}
	for iter.Next() {
		name := strings.TrimPrefix(iter.Selector().String(), "#")
		e, ok := byName[name]
		if !ok {
			continue
		}
		v := iter.Value()
		if v.IncompleteKind() != cue.StructKind {
			continue
		}
		if pos, found := hiddenDTOField(v); found && !reported[pos.String()] {
			reported[pos.String()] = true
			msg := fmt.Sprintf("entity %s: `_dto: true` is not a DTO marker and ANG does not read it; %s is not a DTO through it", name, name)
			if IsDTO(e) {
				msg = fmt.Sprintf("entity %s: `_dto: true` repeats the marker @dto(only=\"true\") and ANG does not read it", name)
			}
			n.Warn(Warning{
				Kind:     "entity",
				Code:     CodeDTOMarkerDeprecated,
				Severity: "warn",
				Message:  msg,
				File:     relativeToWorkingDir(pos.Filename()),
				Line:     pos.Line(),
				Column:   pos.Column(),
				Hint:     "run `ang fix dto`: it removes the field and writes @dto(only=\"true\") on the entity that declares it",
			})
		}
		if IsDTO(e) || entityHasIDField(e) || entityStoredOutsidePostgres(e) {
			continue
		}
		file, line := splitSourcePos(e.Source)
		n.Warn(Warning{
			Kind:     "entity",
			Code:     CodeEntityWithoutID,
			Severity: "warn",
			Message:  fmt.Sprintf("entity %s has no id field and no DTO marker, so it gets no table", name),
			File:     file,
			Line:     line,
			Hint:     "mark a shape that nothing stores @dto(only=\"true\"), or give a stored entity an id field",
		})
	}
}

// hiddenDTOField finds a `_dto` field of an entity value (written in it or
// inherited) and where it is written.
func hiddenDTOField(v cue.Value) (token.Pos, bool) {
	fields, err := v.Fields(cue.Hidden(true))
	if err != nil {
		return token.NoPos, false
	}
	for fields.Next() {
		sel := fields.Selector()
		if sel.LabelType() == cue.HiddenLabel && sel.String() == "_dto" {
			p := fields.Value().Pos()
			return p, p.IsValid()
		}
	}
	return token.NoPos, false
}

func entityHasIDField(e Entity) bool {
	for _, f := range e.Fields {
		if strings.EqualFold(f.Name, "id") {
			return true
		}
	}
	return false
}

// entityStoredOutsidePostgres mirrors the SQL emitter: @storage("mongo") or an
// ObjectId field keeps an entity out of PostgreSQL.
func entityStoredOutsidePostgres(e Entity) bool {
	if storage, ok := e.Metadata["storage"].(string); ok && strings.EqualFold(storage, "mongo") {
		return true
	}
	for _, f := range e.Fields {
		if strings.EqualFold(f.DB.Type, "ObjectId") {
			return true
		}
	}
	return false
}

func splitSourcePos(source string) (string, int) {
	idx := strings.LastIndex(source, ":")
	if idx <= 0 {
		return source, 0
	}
	line := 0
	if _, err := fmt.Sscanf(source[idx+1:], "%d", &line); err != nil {
		return source, 0
	}
	return source[:idx], line
}
