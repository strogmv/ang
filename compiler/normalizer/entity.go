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
	entityVal := val
	entity := Entity{
		Name:        name,
		Description: description,
		Fields:      []Field{},
		Metadata:    map[string]any{},
		Source:      formatPos(val),
	}

	// 1. Explicit owner via "owner" field
	if owner, err := val.LookupPath(cue.ParsePath("owner")).String(); err == nil && owner != "" {
		entity.Owner = owner
	}

	// 2. Explicit owner via @owner attribute
	if attr := val.Attribute("owner"); attr.Err() == nil {
		if s, found, _ := attr.Lookup(0, ""); found {
			entity.Owner = s
		}
	}

	// 3. Deduction from file name if owner is missing
	if entity.Owner == "" {
		if pos := val.Pos(); pos.IsValid() {
			file := pos.Filename()
			base := filepath.Base(file)
			owner := strings.TrimSuffix(base, filepath.Ext(base))
			// Special cases for common shared entities
			if owner == "domain" || owner == "types" || owner == "common" || owner == "entities" {
				entity.Owner = "" // Shared/Universal
			} else {
				entity.Owner = owner
			}
		}
	}

	// 4. Optional storage override via @storage attribute
	if attr := val.Attribute("storage"); attr.Err() == nil {
		if s, found, _ := attr.Lookup(0, ""); found && s != "" {
			entity.Metadata["storage"] = s
		}
	}

	// 5. Check for @dto(only="true") or _dto: true
	if attr := val.Attribute("dto"); attr.Err() == nil {
		if v, found, _ := attr.Lookup(0, "only"); found && v == "true" {
			entity.Metadata["dto"] = true
		}
	}
	if b, err := val.LookupPath(cue.ParsePath("_dto")).Bool(); err == nil && b {
		entity.Metadata["dto"] = true
	}

	// If entity has a 'fields' field, iterate that instead
	fieldsContainer := entityVal
	fieldsVal := entityVal.LookupPath(cue.ParsePath("fields"))
	if fieldsVal.Exists() && fieldsVal.Kind() == cue.StructKind {
		fieldsContainer = fieldsVal
	}

	iter, err := fieldsContainer.Fields(cue.All())
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
			Constraints: extractConstraints(val),
			EnvVar:      parseEnvTag(val),
			UI:          parseUIHints(val),
			Source:      formatPos(val),
		}
		// Align TIMESTAMP/TIMESTAMPTZ columns with time.Time when detection fell back to string.
		if strings.EqualFold(field.Type, "string") && field.DB.Type != "" {
			if t := strings.ToUpper(field.DB.Type); strings.Contains(t, "TIMESTAMP") {
				field.Type = "time.Time"
			}
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
		} else if !strings.HasSuffix(name, "Request") && !strings.HasSuffix(name, "Response") {
			// Auto-detect secrets by field name only for domain entities,
			// not for operation input/output (Request/Response) where fields
			// like "password", "accessToken", "refreshToken" are part of the API contract.
			if strings.Contains(strings.ToLower(fLabel), "password") || strings.Contains(strings.ToLower(fLabel), "token") {
				field.IsSecret = true
			}
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
			if cs, found, _ := attr.Lookup(0, "client_side"); found && cs == "true" {
				field.Metadata["client_side_encryption"] = true
			}
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
	fsmVal := entityVal.LookupPath(cue.ParsePath("fsm"))
	if fsmVal.Exists() {
		fsm := &FSM{
			States:      []string{},
			Transitions: make(map[string][]string),
		}

		if f, err := fsmVal.LookupPath(cue.ParsePath("field")).String(); err == nil {
			fsm.Field = strings.Trim(f, "")
		}

		statesVal := fsmVal.LookupPath(cue.ParsePath("states"))
		if statesVal.Exists() {
			list, _ := statesVal.List()
			for list.Next() {
				s, err := list.Value().String()
				if err != nil {
					continue
				}
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				fsm.States = append(fsm.States, s)
			}
		}

		trVal := fsmVal.LookupPath(cue.ParsePath("transitions"))
		if trVal.Exists() {
			switch trVal.IncompleteKind() {
			case cue.ListKind:
				list, _ := trVal.List()
				for list.Next() {
					tv := list.Value()
					fromState, err := tv.LookupPath(cue.ParsePath("from")).String()
					if err != nil {
						continue
					}
					toState, err := tv.LookupPath(cue.ParsePath("to")).String()
					if err != nil {
						continue
					}
					fromState = strings.TrimSpace(fromState)
					toState = strings.TrimSpace(toState)
					if fromState == "" || toState == "" {
						continue
					}
					fsm.Transitions[fromState] = append(fsm.Transitions[fromState], toState)
				}
			default:
				iter, _ := trVal.Fields()
				for iter.Next() {
					fromState := strings.TrimSpace(iter.Selector().String())
					var toStates []string
					list, _ := iter.Value().List()
					for list.Next() {
						s, err := list.Value().String()
						if err != nil {
							continue
						}
						toStates = append(toStates, strings.TrimSpace(s))
					}
					if fromState == "" || len(toStates) == 0 {
						continue
					}
					fsm.Transitions[fromState] = append(fsm.Transitions[fromState], toStates...)
				}
			}
		}
		entity.FSM = fsm
	}

	indexVal := entityVal.LookupPath(cue.ParsePath("indexes"))
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

	entity.UI = parseEntityUI(entityVal)

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
			Constraints: extractConstraints(fVal),
			EnvVar:      parseEnvTag(fVal),
			UI:          parseUIHints(fVal),
			Source:      formatPos(fVal),
		}

		if attr := fVal.Attribute("secret"); attr.Err() == nil {
			field.IsSecret = true
		}
		// Note: no auto-detect by field name here; parseInlineFields is used
		// for nested data fields inside operations where password/token fields
		// are part of the API contract. Use @secret explicitly if needed.
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
	// 1. Explicit tag (@validate)
	attrs := v.Attributes(cue.ValueAttr)
	for _, attr := range attrs {
		if attr.Name() == "validate" {
			val := attr.Contents()
			val = strings.TrimPrefix(val, "rule=")
			val = strings.Trim(val, "\"")
			return val
		}
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
	if val, found, _ := attr.Lookup(0, "importance"); found {
		hints.Importance = val
	}
	if val, found, _ := attr.Lookup(0, "inputKind"); found {
		hints.InputKind = val
	}
	if val, found, _ := attr.Lookup(0, "intent"); found {
		hints.Intent = val
	}
	if val, found, _ := attr.Lookup(0, "density"); found {
		hints.Density = val
	}
	if val, found, _ := attr.Lookup(0, "labelMode"); found {
		hints.LabelMode = val
	}
	if val, found, _ := attr.Lookup(0, "surface"); found {
		hints.Surface = val
	}
	if val, found, _ := attr.Lookup(0, "component"); found {
		hints.Component = val
	}
	if val, found, _ := attr.Lookup(0, "section"); found {
		hints.Section = val
	}
	if val, found, _ := attr.Lookup(0, "columns"); found {
		if n, err := strconv.Atoi(val); err == nil {
			hints.Columns = n
		}
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

func extractConstraints(v cue.Value) *Constraints {
	c := &Constraints{}
	hasAny := false

	op, args := v.Expr()

	// Recursively handle AND (e.g. >0 & <100)
	if op == cue.AndOp {
		for _, arg := range args {
			sub := extractConstraints(arg)
			if sub != nil {
				if sub.Min != nil {
					c.Min = sub.Min
				}
				if sub.Max != nil {
					c.Max = sub.Max
				}
				if sub.MinLen != nil {
					c.MinLen = sub.MinLen
				}
				if sub.MaxLen != nil {
					c.MaxLen = sub.MaxLen
				}
				if sub.Regex != "" {
					c.Regex = sub.Regex
				}
				if len(sub.Enum) > 0 {
					c.Enum = sub.Enum
				}
				hasAny = true
			}
		}
	}

	switch op {
	case cue.GreaterThanOp, cue.GreaterThanEqualOp:
		val, _ := args[0].Float64()
		c.Min = &val
		hasAny = true
	case cue.LessThanOp, cue.LessThanEqualOp:
		val, _ := args[0].Float64()
		c.Max = &val
		hasAny = true
	case cue.CallOp:
		// Handle built-ins like strings.MinRunes(5)
		name := fmt.Sprint(args[0])
		if strings.Contains(name, "MinRunes") {
			v, _ := args[1].Int64()
			iv := int(v)
			c.MinLen = &iv
			hasAny = true
		} else if strings.Contains(name, "MaxRunes") {
			v, _ := args[1].Int64()
			iv := int(v)
			c.MaxLen = &iv
			hasAny = true
		}
	case cue.OrOp:
		// Handle enums: "a" | "b" | "c"
		isEnum := true
		var enum []string
		for _, arg := range args {
			if s, err := arg.String(); err == nil {
				enum = append(enum, s)
			} else {
				isEnum = false
				break
			}
		}
		if isEnum && len(enum) > 0 {
			c.Enum = enum
			hasAny = true
		}
	}

	if !hasAny {
		return nil
	}
	return c
}
