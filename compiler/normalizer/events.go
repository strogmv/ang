package normalizer

import (
	"strings"

	"cuelang.org/go/cue"
)

// ExtractEvents extracts event definitions from a dedicated cue/events package.
func (n *Normalizer) ExtractEvents(val cue.Value) ([]EventDef, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}
	var events []EventDef

	iter, err := val.Fields(cue.All())
	if err != nil {
		return nil, err
	}

	for iter.Next() {
		label := iter.Selector().String()
		value := iter.Value()

		if !strings.HasPrefix(label, "#") {
			continue
		}

		cleanName := strings.TrimPrefix(label, "#")
		if isEventMetaDefinition(cleanName) {
			continue
		}

		ent, err := n.parseEntity(cleanName, value)
		if err != nil {
			return nil, err
		}

		fields := make([]Field, 0, len(ent.Fields))
		for _, f := range ent.Fields {
			name := strings.TrimSpace(strings.ToLower(f.Name))
			if name == "owner" || name == "consumers" {
				continue
			}
			fields = append(fields, f)
		}

		evt := EventDef{
			Name:      ent.Name,
			Owner:     strings.TrimSpace(ent.Owner),
			Consumers: parseStringList(value.LookupPath(cue.ParsePath("consumers"))),
			Fields:    fields,
			Source:    formatPos(value),
		}
		events = append(events, evt)
	}

	return events, nil
}

// ExtractEventsFromArch extracts event definitions from #Events in architecture package.
// Events are defined as: #Events: { EventName: { description: "...", payload: { field: "type" } } }
func (n *Normalizer) ExtractEventsFromArch(val cue.Value) ([]EventDef, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}

	eventsVal := val.LookupPath(cue.ParsePath("#Events"))
	if !eventsVal.Exists() {
		return nil, nil
	}

	var events []EventDef
	iter, err := eventsVal.Fields()
	if err != nil {
		return nil, err
	}

	for iter.Next() {
		name := iter.Selector().String()
		name = cleanName(name)
		if isEventMetaDefinition(name) {
			continue
		}
		v := iter.Value()

		payloadVal := v.LookupPath(cue.ParsePath("payload"))
		var fields []Field
		if payloadVal.Exists() {
			pIter, pErr := payloadVal.Fields()
			if pErr == nil {
				for pIter.Next() {
					fieldName := pIter.Selector().String()
					fieldName = cleanName(fieldName)
					fieldType, _ := pIter.Value().String()
					fields = append(fields, Field{
						Name: fieldName,
						Type: fieldType,
					})
				}
			}
		}

		events = append(events, EventDef{
			Name:      name,
			Owner:     strings.TrimSpace(getString(v, "owner")),
			Consumers: parseStringList(v.LookupPath(cue.ParsePath("consumers"))),
			Fields:    fields,
			Source:    formatPos(v),
		})
	}

	return events, nil
}

func isEventMetaDefinition(name string) bool {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "eventannotations", "eventsmeta", "eventmeta":
		return true
	default:
		return false
	}
}

// ExtractErrors extracts error definitions.
func (n *Normalizer) ExtractErrors(val cue.Value) ([]ErrorDef, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}
	var errs []ErrorDef

	errsVal := val.LookupPath(cue.ParsePath("#Errors"))
	if !errsVal.Exists() {
		return nil, nil
	}

	iter, _ := errsVal.Fields()
	for iter.Next() {
		name := iter.Selector().String()
		name = cleanName(name)
		v := iter.Value()

		code, _ := v.LookupPath(cue.ParsePath("code")).Int64()
		httpStatus, _ := v.LookupPath(cue.ParsePath("http")).Int64()
		msg, _ := v.LookupPath(cue.ParsePath("msg")).String()
		msg = strings.Trim(msg, "")

		errs = append(errs, ErrorDef{
			Name:       name,
			Code:       int(code),
			HTTPStatus: int(httpStatus),
			Message:    msg,
			Source:     formatPos(v),
		})
	}
	return errs, nil
}
