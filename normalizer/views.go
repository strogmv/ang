package normalizer

import (
	"strings"

	"cuelang.org/go/cue"
)

// ExtractViews extracts view definitions from cue/views.
func (n *Normalizer) ExtractViews(val cue.Value) ([]ViewDef, error) {
	var views []ViewDef

	iter, err := val.Fields(cue.All())
	if err != nil {
		return nil, err
	}

	for iter.Next() {
		viewName := strings.TrimSpace(iter.Selector().String())
		viewVal := iter.Value()
		v := ViewDef{
			Name:  viewName,
			Roles: make(map[string][]string),
		}

		roleIter, _ := viewVal.Fields(cue.All())
		for roleIter.Next() {
			roleName := strings.TrimSpace(roleIter.Selector().String())
			roleVal := roleIter.Value()
			fieldIter, _ := roleVal.Fields(cue.All())
			for fieldIter.Next() {
				fieldName := strings.TrimSpace(fieldIter.Selector().String())
				if fieldName == "" || strings.HasPrefix(fieldName, "_") {
					continue
				}
				v.Roles[roleName] = append(v.Roles[roleName], fieldName)
			}
		}

		views = append(views, v)
	}

	return views, nil
}
