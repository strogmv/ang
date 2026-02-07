package normalizer

import (
	"strings"

	"cuelang.org/go/cue"
)

// ExtractEvents extracts event definitions.
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

		ent, err := n.parseEntity(cleanName, value)
		if err != nil {
			return nil, err
		}

		events = append(events, EventDef{
			Name:   ent.Name,
			Fields: ent.Fields,
			Source: formatPos(value),
		})
	}

	return events, nil
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