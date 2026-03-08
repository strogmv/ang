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

	// Bounded context source of truth:
	// 1) @bounded_context("tender")
	// 2) bounded_context: "tender"
	// 3) inferred from owner/file as fallback
	for _, attr := range val.Attributes(cue.ValueAttr | cue.FieldAttr | cue.DeclAttr) {
		if attr.Name() != "bounded_context" && attr.Name() != "boundedContext" {
			continue
		}
		if s, found, _ := attr.Lookup(0, ""); found {
			entity.BoundedContext = strings.TrimSpace(strings.ToLower(s))
			break
		}
	}
	if entity.BoundedContext == "" {
		if bc, err := val.LookupPath(cue.ParsePath("bounded_context")).String(); err == nil {
			entity.BoundedContext = strings.TrimSpace(strings.ToLower(bc))
		}
	}
	if entity.BoundedContext == "" {
		if bc, err := val.LookupPath(cue.ParsePath("boundedContext")).String(); err == nil {
			entity.BoundedContext = strings.TrimSpace(strings.ToLower(bc))
		}
	}
	if entity.BoundedContext == "" {
		entity.BoundedContext = inferBoundedContext(entity.Owner)
	}

	// Aggregate ownership metadata:
	// 1) root: true
	// 2) owns: ["ChildEntityA", "ChildEntityB"]
	// 3) @aggregate(root=true, owns="A,B")
	if b, err := val.LookupPath(cue.ParsePath("root")).Bool(); err == nil {
		entity.AggregateRoot = b
	}
	if ownsVal := val.LookupPath(cue.ParsePath("owns")); ownsVal.Exists() {
		switch ownsVal.IncompleteKind() {
		case cue.ListKind:
			list, _ := ownsVal.List()
			for list.Next() {
				if s, err := list.Value().String(); err == nil {
					s = strings.TrimSpace(s)
					if s != "" {
						entity.Owns = append(entity.Owns, s)
					}
				}
			}
		default:
			if s, err := ownsVal.String(); err == nil {
				s = strings.TrimSpace(s)
				if s != "" {
					entity.Owns = append(entity.Owns, s)
				}
			}
		}
	}
	if attr := val.Attribute("aggregate"); attr.Err() == nil {
		if v, found, _ := attr.Lookup(0, "root"); found {
			if b, err := strconv.ParseBool(v); err == nil {
				entity.AggregateRoot = b
			}
		}
		if v, found, _ := attr.Lookup(0, "owns"); found {
			for _, part := range strings.Split(v, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					entity.Owns = append(entity.Owns, part)
				}
			}
		}
	}
	if len(entity.Owns) > 0 {
		seenOwns := make(map[string]struct{}, len(entity.Owns))
		dedup := make([]string, 0, len(entity.Owns))
		for _, owned := range entity.Owns {
			if _, ok := seenOwns[owned]; ok {
				continue
			}
			seenOwns[owned] = struct{}{}
			dedup = append(dedup, owned)
		}
		entity.Owns = dedup
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

	// 6a. Shared architecture flag: @shared_arch or shared_arch: true in CUE.
	//     Marks entity as cross-service accessible (no ARCHITECTURE_VIOLATION).
	//     Set @shared_arch on the entity definition in cue/domain/*.cue.
	sharedArch := false
	for _, attr := range val.Attributes(cue.ValueAttr | cue.FieldAttr | cue.DeclAttr) {
		if attr.Name() != "shared_arch" {
			continue
		}
		sharedArch = true
		if reason, found, _ := attr.Lookup(0, "reason"); found {
			reason = strings.TrimSpace(reason)
			if reason != "" {
				entity.Metadata["shared_arch_reason"] = reason
			}
		}
		if ticket, found, _ := attr.Lookup(0, "ticket"); found {
			ticket = strings.TrimSpace(ticket)
			if ticket != "" {
				entity.Metadata["shared_arch_ticket"] = ticket
			}
		}
	}
	if b, err := val.LookupPath(cue.ParsePath("shared_arch")).Bool(); err == nil && b {
		sharedArch = true
	}
	if sharedVal := val.LookupPath(cue.ParsePath("shared_arch")); sharedVal.Exists() && sharedVal.IncompleteKind() == cue.StructKind {
		if b, err := sharedVal.LookupPath(cue.ParsePath("enabled")).Bool(); err == nil && b {
			sharedArch = true
		}
		if reason, err := sharedVal.LookupPath(cue.ParsePath("reason")).String(); err == nil {
			reason = strings.TrimSpace(reason)
			if reason != "" {
				entity.Metadata["shared_arch_reason"] = reason
			}
		}
		if ticket, err := sharedVal.LookupPath(cue.ParsePath("ticket")).String(); err == nil {
			ticket = strings.TrimSpace(ticket)
			if ticket != "" {
				entity.Metadata["shared_arch_ticket"] = ticket
			}
		}
	}
	if reason, err := val.LookupPath(cue.ParsePath("shared_arch_reason")).String(); err == nil {
		reason = strings.TrimSpace(reason)
		if reason != "" {
			entity.Metadata["shared_arch_reason"] = reason
		}
	}
	if ticket, err := val.LookupPath(cue.ParsePath("shared_arch_ticket")).String(); err == nil {
		ticket = strings.TrimSpace(ticket)
		if ticket != "" {
			entity.Metadata["shared_arch_ticket"] = ticket
		}
	}
	if sharedArch {
		entity.Metadata["shared_arch"] = true
	}

	// 6. Read-model contract for ACL analytics projections.
	if rm := parseReadModelDef(val); rm != nil {
		entity.ReadModel = rm
		entity.Metadata["read_model"] = true
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

		if fLabel == "fsm" || fLabel == "indexes" || fLabel == "methods" || fLabel == "owner" || fLabel == "root" || fLabel == "owns" || fLabel == "bounded_context" || fLabel == "boundedContext" || fLabel == "read_model" || fLabel == "readModel" || fLabel == "refreshOn" {
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
