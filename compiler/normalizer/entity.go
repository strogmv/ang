package normalizer

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
)

// ExtractEntities scans a CUE value and extracts struct definitions (starting with #).
func (n *Normalizer) ExtractEntities(val cue.Value) ([]Entity, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}
	var entities []Entity

	iter, err := val.Fields(cue.All())
	if err != nil {
		return nil, err
	}

	for iter.Next() {
		label := iter.Selector().String()
		value := iter.Value()

		if strings.HasPrefix(label, "#") {
			if strings.HasSuffix(label, "Service") || strings.HasSuffix(label, "API") || label == "#AppConfig" || label == "#RBAC" {
				continue
			}
			label = strings.TrimPrefix(label, "#")
		}
		if strings.HasSuffix(label, "Service") || strings.HasSuffix(label, "API") || label == "AppConfig" || label == "RBAC" {
			continue
		}
		if strings.HasPrefix(label, "_") {
			continue
		}
		entity, err := n.parseEntity(label, value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse entity %s: %w", label, err)
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

func (n *Normalizer) parseEntity(name string, val cue.Value) (Entity, error) {
	description, _ := val.LookupPath(cue.ParsePath("description")).String()
	entity := Entity{
		Name:        name,
		Description: description,
		Fields:      []Field{},
		Metadata:    map[string]any{},
		Source:      formatPos(val),
	}

	// 1. Explicit owner via @owner attribute
	if attr := val.Attribute("owner"); attr.Err() == nil {
		if s, found, _ := attr.Lookup(0, ""); found {
			entity.Owner = s
		}
	}

	// 2. Deduction from file name if owner is missing
	if entity.Owner == "" {
		if pos := val.Pos(); pos.IsValid() {
			file := pos.Filename()
			base := filepath.Base(file)
			owner := strings.TrimSuffix(base, filepath.Ext(base))
			// Special cases for common shared entities
			if owner == "domain" || owner == "types" || owner == "common" {
				entity.Owner = "" // Shared/Universal
			} else {
				entity.Owner = owner
			}
		}
	}

	// 3. Optional storage override via @storage attribute
	if attr := val.Attribute("storage"); attr.Err() == nil {
		if s, found, _ := attr.Lookup(0, ""); found && s != "" {
			entity.Metadata["storage"] = s
		}
	}

	// 4. Check for @dto(only="true") or _dto: true
	if attr := val.Attribute("dto"); attr.Err() == nil {
		if v, found, _ := attr.Lookup(0, "only"); found && v == "true" {
			entity.Metadata["dto"] = true
		}
	}
	if b, err := val.LookupPath(cue.ParsePath("_dto")).Bool(); err == nil && b {
		entity.Metadata["dto"] = true
	}

	iter, err := val.Fields(cue.All())
	if err != nil {
		return entity, err
	}

	for iter.Next() {
		fLabel := iter.Selector().String()
		fLabel = cleanName(fLabel)

		if fLabel == "fsm" || fLabel == "indexes" || fLabel == "methods" {
			continue
		}

		val := iter.Value()
		var defVal string

		dVal, _ := val.Default()
		if dVal.IsConcrete() && (dVal.IncompleteKind() != cue.StructKind && dVal.IncompleteKind() != cue.ListKind) {
			defVal = fmt.Sprint(dVal)
		}

		field := Field{
			Name:        fLabel,
			IsOptional:  iter.IsOptional(),
			Type:        n.detectType(fLabel, val),
			Default:     defVal,
			DB:          parseDBTags(val),
			ValidateTag: inferValidatorTags(fLabel, val),
			EnvVar:      parseEnvTag(val),
			UI:          parseUIHints(val),
			Source:      formatPos(val),
		}

		// Field level SkipDomain logic
		if fLabel == "ui" {
			field.SkipDomain = true
		}
		if attr := val.Attribute("dto"); attr.Err() == nil {
			if v, found, _ := attr.Lookup(0, "only"); found && v == "true" {
				field.SkipDomain = true
			}
		}

		if attr := val.Attribute("secret"); attr.Err() == nil {
			field.IsSecret = true
		} else if strings.Contains(strings.ToLower(fLabel), "password") || strings.Contains(strings.ToLower(fLabel), "token") {
			field.IsSecret = true
		}

		if attr := val.Attribute("pii"); attr.Err() == nil {
			field.IsPII = true
			if cls, found, _ := attr.Lookup(0, "classification"); found {
				if field.Metadata == nil {
					field.Metadata = make(map[string]any)
				}
				field.Metadata["pii_classification"] = cls
			}
		}

		if attr := val.Attribute("encrypt"); attr.Err() == nil {
			if field.Metadata == nil {
				field.Metadata = make(map[string]any)
			}
			mode := "randomized"
			if m, found, _ := attr.Lookup(0, "mode"); found {
				mode = m
			}
			field.Metadata["encrypt"] = mode
		}

		if attr := val.Attribute("redact"); attr.Err() == nil {
			if field.Metadata == nil {
				field.Metadata = make(map[string]any)
			}
			field.Metadata["redact"] = true
		}

		if attr := val.Attribute("image"); attr.Err() == nil {
			field.FileMeta = &FileMeta{Kind: "image", Thumbnail: true}
		} else if attr := val.Attribute("file"); attr.Err() == nil {
			kind := "auto"
			if k, found, _ := attr.Lookup(0, "kind"); found {
				kind = k
			}
			thumb := false
			if t, found, _ := attr.Lookup(0, "thumbnail"); found {
				if b, err := strconv.ParseBool(t); err == nil {
					thumb = b
				}
			}
			field.FileMeta = &FileMeta{Kind: kind, Thumbnail: thumb}
		}
		if val.IncompleteKind() == cue.ListKind {
			field.IsList = true
			if strings.HasPrefix(field.Type, "[]domain.") {
				field.ItemTypeName = strings.TrimPrefix(field.Type, "[]domain.")
			}

			anyElem := val.LookupPath(cue.MakePath(cue.AnyIndex))
			if anyElem.Exists() && anyElem.IncompleteKind() == cue.StructKind {
				_, path := anyElem.ReferencePath()
				if len(path.Selectors()) == 0 {
					itemName := exportName(name) + exportName(fLabel) + "Item"
					if strings.EqualFold(fLabel, "data") {
						itemName = exportName(name) + "Data"
					}
					itemFields, err := n.parseInlineFields(anyElem)
					if err == nil && len(itemFields) > 0 {
						field.Type = "[]" + itemName
						field.ItemTypeName = itemName
						field.ItemFields = itemFields
					}
				}
			}
		}
		entity.Fields = append(entity.Fields, field)
	}

	// Parse FSM
	fsmVal := val.LookupPath(cue.ParsePath("fsm"))
	if fsmVal.Exists() {
		fsm := &FSM{
			Transitions: make(map[string][]string),
		}

		if f, err := fsmVal.LookupPath(cue.ParsePath("field")).String(); err == nil {
			fsm.Field = strings.Trim(f, "")
		}

		trVal := fsmVal.LookupPath(cue.ParsePath("transitions"))
		if trVal.Exists() {
			iter, _ := trVal.Fields()
			for iter.Next() {
				fromState := strings.Trim(iter.Selector().String(), "")
				var toStates []string
				list, _ := iter.Value().List()
				for list.Next() {
					s, _ := list.Value().String()
					toStates = append(toStates, strings.Trim(s, ""))
				}
				fsm.Transitions[fromState] = toStates
			}
		}
		entity.FSM = fsm
	}

	indexVal := val.LookupPath(cue.ParsePath("indexes"))
	if indexVal.Exists() {
		iter, _ := indexVal.List()
		for iter.Next() {
			iv := iter.Value()
			var fields []string
			fv := iv.LookupPath(cue.ParsePath("fields"))
			if fv.Exists() {
				fit, _ := fv.List()
				for fit.Next() {
					s, _ := fit.Value().String()
					s = strings.TrimSpace(s)
					if s != "" {
						fields = append(fields, s)
					}
				}
			}
			if len(fields) == 0 {
				continue
			}
			unique := false
			if v, err := iv.LookupPath(cue.ParsePath("unique")).Bool(); err == nil {
				unique = v
			}
			entity.Indexes = append(entity.Indexes, IndexDef{
				Fields: fields,
				Unique: unique,
			})
		}
	}

	entity.UI = parseEntityUI(val)

	return entity, nil
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

func (n *Normalizer) parseInlineFields(val cue.Value) ([]Field, error) {
	var fields []Field
	iter, err := val.Fields(cue.All())
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		fLabel := cleanName(iter.Selector().String())
		if fLabel == "" {
			continue
		}
		fVal := iter.Value()
		var defVal string
		dVal, _ := fVal.Default()
		if dVal.IsConcrete() && (dVal.IncompleteKind() != cue.StructKind && dVal.IncompleteKind() != cue.ListKind) {
			defVal = fmt.Sprint(dVal)
		}
		field := Field{
			Name:        fLabel,
			IsOptional:  iter.IsOptional(),
			Type:        n.detectType(fLabel, fVal),
			Default:     defVal,
			DB:          parseDBTags(fVal),
			ValidateTag: inferValidatorTags(fLabel, fVal),
			EnvVar:      parseEnvTag(fVal),
			UI:          parseUIHints(fVal),
			Source:      formatPos(fVal),
		}
		if attr := fVal.Attribute("secret"); attr.Err() == nil {
			field.IsSecret = true
		} else if strings.Contains(strings.ToLower(fLabel), "password") {
			field.IsSecret = true
		}
		if attr := fVal.Attribute("pii"); attr.Err() == nil {
			field.IsPII = true
		}
		if attr := fVal.Attribute("encrypt"); attr.Err() == nil {
			if field.Metadata == nil {
				field.Metadata = make(map[string]any)
			}
			mode := "randomized"
			if m, found, _ := attr.Lookup(0, "mode"); found {
				mode = m
			}
			field.Metadata["encrypt"] = mode
		}
		if attr := fVal.Attribute("redact"); attr.Err() == nil {
			if field.Metadata == nil {
				field.Metadata = make(map[string]any)
			}
			field.Metadata["redact"] = true
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func parseEnvTag(v cue.Value) string {
	attr := v.Attribute("env")
	if attr.Err() == nil {
		val := attr.Contents()
		return strings.Trim(val, "\"")
	}
	return ""
}

func inferValidatorTags(name string, v cue.Value) string {
	// 1. Explicit tag
	attr := v.Attribute("validate")
	if attr.Err() == nil {
		val := attr.Contents()
		return strings.Trim(val, "")
	}

	// 2. Heuristic (Auto-Discovery)
	name = strings.ToLower(name)
	if name == "email" {
		return "email"
	}
	if strings.Contains(name, "url") {
		return "url"
	}

	return ""
}

func parseDBTags(v cue.Value) DBMeta {
	meta := DBMeta{
		Type: "TEXT", // Default
	}

	attr := v.Attribute("db")
	if err := attr.Err(); err != nil {
		return meta
	}

	if val, found, _ := attr.Lookup(0, "type"); found {
		meta.Type = val
	}

	if _, found, _ := attr.Lookup(0, "primary_key"); found {
		meta.PrimaryKey = true
	}

	if _, found, _ := attr.Lookup(0, "unique"); found {
		meta.Unique = true
	}

	if _, found, _ := attr.Lookup(0, "index"); found {
		meta.Index = true
	}

	return meta
}

func parseUIHints(v cue.Value) *UIHints {
	attr := v.Attribute("ui")
	if err := attr.Err(); err != nil {
		return nil
	}

	hints := &UIHints{
		FullWidth: true, // default
	}

	if val, found, _ := attr.Lookup(0, "type"); found {
		hints.Type = val
	}
	if val, found, _ := attr.Lookup(0, "label"); found {
		hints.Label = val
	}
	if val, found, _ := attr.Lookup(0, "placeholder"); found {
		hints.Placeholder = val
	}
	if val, found, _ := attr.Lookup(0, "helperText"); found {
		hints.HelperText = val
	}
	if val, found, _ := attr.Lookup(0, "order"); found {
		if n, err := strconv.Atoi(val); err == nil {
			hints.Order = n
		}
	}
	if _, found, _ := attr.Lookup(0, "hidden"); found {
		hints.Hidden = true
	}
	if _, found, _ := attr.Lookup(0, "disabled"); found {
		hints.Disabled = true
	}
	if val, found, _ := attr.Lookup(0, "fullWidth"); found {
		hints.FullWidth = val != "false"
	}
	if val, found, _ := attr.Lookup(0, "rows"); found {
		if n, err := strconv.Atoi(val); err == nil {
			hints.Rows = n
		}
	}
	if val, found, _ := attr.Lookup(0, "min"); found {
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			hints.Min = &n
		}
	}
	if val, found, _ := attr.Lookup(0, "max"); found {
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			hints.Max = &n
		}
	}
	if val, found, _ := attr.Lookup(0, "step"); found {
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			hints.Step = &n
		}
	}
	if val, found, _ := attr.Lookup(0, "currency"); found {
		hints.Currency = val
	}
	if val, found, _ := attr.Lookup(0, "source"); found {
		hints.Source = val
	}
	if val, found, _ := attr.Lookup(0, "accept"); found {
		hints.Accept = val
	}
	if val, found, _ := attr.Lookup(0, "maxSize"); found {
		if n, err := strconv.Atoi(val); err == nil {
			hints.MaxSize = n
		}
	}
	if _, found, _ := attr.Lookup(0, "multiple"); found {
		hints.Multiple = true
	}

	return hints
}
