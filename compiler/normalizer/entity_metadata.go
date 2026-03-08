package normalizer

import (
	"strings"

	"cuelang.org/go/cue"
)

func inferBoundedContext(owner string) string {
	owner = strings.TrimSpace(strings.ToLower(owner))
	if owner == "" {
		return ""
	}
	for _, sep := range []string{"_", "-", "."} {
		if i := strings.Index(owner, sep); i > 0 {
			return owner[:i]
		}
	}
	return owner
}

func parseReadModelDef(val cue.Value) *ReadModelDef {
	var src string
	var refreshOn []string
	enabled := false

	parseBlock := func(v cue.Value) {
		if !v.Exists() {
			return
		}
		enabled = true
		if s, err := v.LookupPath(cue.ParsePath("source_context")).String(); err == nil && strings.TrimSpace(s) != "" {
			src = strings.TrimSpace(strings.ToLower(s))
		}
		if s, err := v.LookupPath(cue.ParsePath("sourceContext")).String(); err == nil && strings.TrimSpace(s) != "" {
			src = strings.TrimSpace(strings.ToLower(s))
		}
		if list := parseStringList(v.LookupPath(cue.ParsePath("refreshOn"))); len(list) > 0 {
			refreshOn = list
		}
	}

	parseBlock(val.LookupPath(cue.ParsePath("read_model")))
	parseBlock(val.LookupPath(cue.ParsePath("readModel")))

	if attr := val.Attribute("read_model"); attr.Err() == nil {
		enabled = true
		if s, found, _ := attr.Lookup(0, "source_context"); found && strings.TrimSpace(s) != "" {
			src = strings.TrimSpace(strings.ToLower(s))
		}
		if s, found, _ := attr.Lookup(0, "sourceContext"); found && strings.TrimSpace(s) != "" {
			src = strings.TrimSpace(strings.ToLower(s))
		}
		if s, found, _ := attr.Lookup(0, "refresh_on"); found && strings.TrimSpace(s) != "" {
			parts := strings.Split(s, ",")
			out := make([]string, 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					out = append(out, part)
				}
			}
			if len(out) > 0 {
				refreshOn = out
			}
		}
	}

	if !enabled {
		return nil
	}
	return &ReadModelDef{
		SourceContext: src,
		RefreshOn:     refreshOn,
	}
}

func parseEntityUI(val cue.Value) *EntityUIDef {
	uiVal := val.LookupPath(cue.ParsePath("ui"))
	if !uiVal.Exists() {
		// Try attribute @ui
		attr := val.Attribute("ui")
		if attr.Err() != nil {
			return nil
		}
		// If it's just @ui(crud), we might need more complex parsing
		// For now, let's look for explicit 'ui' struct in CUE
		return nil
	}

	res := &EntityUIDef{}

	crudVal := uiVal.LookupPath(cue.ParsePath("crud"))
	if crudVal.Exists() {
		crud := &CRUDDef{
			Views: map[string]bool{
				"list":    true,
				"details": true,
				"create":  true,
				"edit":    true,
			},
			Perms: make(map[string]string),
		}
		if v, err := crudVal.LookupPath(cue.ParsePath("enabled")).Bool(); err == nil {
			crud.Enabled = v
		}
		if v, err := crudVal.LookupPath(cue.ParsePath("custom")).Bool(); err == nil {
			crud.Custom = v
		}

		// Parse views
		viewsVal := crudVal.LookupPath(cue.ParsePath("views"))
		if viewsVal.Exists() {
			it, _ := viewsVal.Fields()
			for it.Next() {
				if b, err := it.Value().Bool(); err == nil {
					crud.Views[it.Selector().String()] = b
				}
			}
		}

		// Parse perms
		permsVal := crudVal.LookupPath(cue.ParsePath("permissions"))
		if permsVal.Exists() {
			it, _ := permsVal.Fields()
			for it.Next() {
				if s, err := it.Value().String(); err == nil {
					crud.Perms[it.Selector().String()] = s
				}
			}
		}
		res.CRUD = crud
	}

	return res
}
