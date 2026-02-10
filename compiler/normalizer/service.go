package normalizer

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// ExtractServices extracts service definitions.
func (n *Normalizer) ExtractServices(val cue.Value, entities []Entity) ([]Service, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}
	var services []Service

	entityOwners := make(map[string]string)
	isDTO := make(map[string]bool)
	for _, e := range entities {
		entityOwners[e.Name] = e.Owner
		if dto, ok := e.Metadata["dto"].(bool); ok && dto {
			isDTO[e.Name] = true
		}
	}

	cacheByOp := make(map[string]struct {
		ttl  string
		tags []string
	})
	httpVal := val.LookupPath(cue.ParsePath("HTTP"))
	if httpVal.Exists() {
		hIter, _ := httpVal.Fields()
		for hIter.Next() {
			opName := cleanName(hIter.Selector().String())
			ttl := getString(hIter.Value(), "cache.ttl")
			var tags []string
			tagsVal := hIter.Value().LookupPath(cue.ParsePath("cache.tags"))
			if tagsVal.Exists() {
				it, _ := tagsVal.List()
				for it.Next() {
					s, _ := it.Value().String()
					tags = append(tags, s)
				}
			}
			if ttl != "" || len(tags) > 0 {
				cacheByOp[opName] = struct {
					ttl  string
					tags []string
				}{ttl, tags}
			}
		}
	}

	serviceMap := make(map[string]*Service)
	iter, err := val.Fields(cue.All())
	if err != nil {
		return nil, err
	}

	for iter.Next() {
		label := iter.Selector().String()
		value := iter.Value()

		if strings.HasPrefix(label, "#") || label == "HTTP" {
			continue
		}

		opName := cleanName(label)
		svcNameRaw := getString(value, "service")
		if svcNameRaw == "" {
			continue
		}
		svcName := normalizeServiceName(svcNameRaw)

		svc, ok := serviceMap[svcName]
		if !ok {
			svc = &Service{
				Name:       svcName,
				Subscribes: make(map[string]string),
				Uses:       []string{},
				Source:     formatPos(value),
			}
			serviceMap[svcName] = svc
		}

		if v, err := value.LookupPath(cue.ParsePath("requiresS3")).Bool(); err == nil && v {
			svc.RequiresS3 = true
		}

		method := Method{
			Name:   opName,
			Source: formatPos(value),
		}
		if info, ok := cacheByOp[opName]; ok {
			method.CacheTTL = info.ttl
			method.CacheTags = info.tags
			if method.CacheTTL != "" {
				svc.RequiresRedis = true
			}
		}

		// Find attributes robustly (Value, Field, or Declaration level)
		attrs := value.Attributes(cue.ValueAttr)
		fattrs := value.Attributes(cue.FieldAttr)
		dattrs := value.Attributes(cue.DeclAttr)

		allAttrs := append(attrs, fattrs...)
		allAttrs = append(allAttrs, dattrs...)

		method.Attributes = parseAttributes(value)

		for _, attr := range allAttrs {
			switch attr.Name() {
			case "idempotent":
				method.Idempotency = true
			case "dedupeKey":
				// Collect all arguments for composite deduplication key
				var keys []string
				for i := 0; ; i++ {
					if s, found, _ := attr.Lookup(i, ""); found {
						keys = append(keys, s)
					} else {
						break
					}
				}
				method.DedupeKey = strings.Join(keys, ", ")
			case "outbox":
				method.Outbox = true
				svc.RequiresSQL = true
			case "audit":
				if method.Metadata == nil {
					method.Metadata = make(map[string]any)
				}
				method.Metadata["audit"] = true
				if event, found, _ := attr.Lookup(0, ""); found {
					method.Metadata["audit_event"] = event
				}
			}
		}

		// Extract test hints
		thVal := value.LookupPath(cue.ParsePath("testHints"))
		if thVal.Exists() {
			if method.Metadata == nil {
				method.Metadata = make(map[string]any)
			}
			method.Metadata["testHints"] = true
		}

		inVal := value.LookupPath(cue.ParsePath("input"))
		if !inVal.Exists() {
			inVal = value.LookupPath(cue.ParsePath("in"))
		}
		if inVal.Exists() {
			ent, err := n.parseEntity(opName+"Request", inVal)
			if err != nil {
				return nil, fmt.Errorf("failed to parse input for %s: %w", opName, err)
			}
			method.Input = ent
		}

		outVal := value.LookupPath(cue.ParsePath("output"))
		if !outVal.Exists() {
			outVal = value.LookupPath(cue.ParsePath("out"))
		}
		if outVal.Exists() {
			ent, err := n.parseEntity(opName+"Response", outVal)
			if err != nil {
				return nil, fmt.Errorf("failed to parse output for %s: %w", opName, err)
			}
			method.Output = ent
		}

		// Analyze sources
		srcVal := value.LookupPath(cue.ParsePath("sources"))
		if srcVal.Exists() {
			srcIter, _ := srcVal.Fields()
			for srcIter.Next() {
				sName := srcIter.Selector().String()
				sVal := srcIter.Value()

				kind := getString(sVal, "kind")
				entName := getString(sVal, "entity")
				if entName != "" && kind == "sql" {
					if _, ok := entityOwners[entName]; !ok {
						n.Warn(Warning{
							Kind:     "architecture",
							Code:     "UNKNOWN_ENTITY",
							Severity: "error",
							Message:  fmt.Sprintf("Source '%s' in operation '%s' refers to unknown entity '%s'", sName, opName, entName),
							Hint:     "Define the entity in cue/domain/ or check spelling",
							CUEPath:  sVal.Path().String(),
						})
					} else if isDTO[entName] {
						n.Warn(Warning{
							Kind:     "architecture",
							Code:     "DTO_AS_REPO",
							Severity: "error",
							Message:  fmt.Sprintf("Source '%s' in operation '%s' refers to DTO-only entity '%s'", sName, opName, entName),
							Hint:     "Repository access is not allowed for DTOs. Remove @dto(only=true) or use a real domain entity",
							CUEPath:  sVal.Path().String(),
						})
					}
				}

				source := Source{
					Name:       sName,
					Kind:       kind,
					Entity:     entName,
					Collection: getString(sVal, "collection"),
					By:         make(map[string]string),
					Filter:     make(map[string]string),
				}

				switch kind {
				case "sql":
					svc.RequiresSQL = true
				case "mongo":
					svc.RequiresMongo = true
				case "redis":
					svc.RequiresRedis = true
				case "s3":
					svc.RequiresS3 = true
				}

				byVal := sVal.LookupPath(cue.ParsePath("by"))
				if byVal.Exists() {
					bit, _ := byVal.Fields()
					for bit.Next() {
						v, _ := bit.Value().String()
						source.By[bit.Selector().String()] = strings.TrimSpace(v)
					}
				}

				filterVal := sVal.LookupPath(cue.ParsePath("filter"))
				if filterVal.Exists() {
					fit, _ := filterVal.Fields()
					for fit.Next() {
						v, _ := fit.Value().String()
						source.Filter[fit.Selector().String()] = strings.TrimSpace(v)
					}
				}

				method.Sources = append(method.Sources, source)
			}
		}

		// Service dependencies
		usesVal := value.LookupPath(cue.ParsePath("uses"))
		if usesVal.Exists() {
			it, _ := usesVal.List()
			for it.Next() {
				raw, _ := it.Value().String()
				if strings.TrimSpace(raw) == "" {
					continue
				}
				dep := normalizeServiceName(raw)
				if dep == svcName {
					continue
				}
				already := false
				for _, existing := range svc.Uses {
					if existing == dep {
						already = true
						break
					}
				}
				if !already {
					svc.Uses = append(svc.Uses, dep)
				}
			}
		}

		// Look for implementation
		var implVal cue.Value
		if iv := value.LookupPath(cue.ParsePath("impls")); iv.Exists() {
			if gv := iv.LookupPath(cue.ParsePath("go")); gv.Exists() {
				implVal = gv
			}
		}
		if !implVal.Exists() {
			implVal = value.LookupPath(cue.ParsePath("_impl"))
		}
		if implVal.Exists() && implVal.IncompleteKind() == cue.BottomKind {
			implVal = cue.Value{}
		}
		if !implVal.Exists() {
			implVal = value.LookupPath(cue.ParsePath("impl"))
		}

		if implVal.Exists() {
			codeVal := implVal.LookupPath(cue.ParsePath("code"))
			code, _ := codeVal.String()

			if code != "" {
				impl := &MethodImpl{
					Lang:       getString(implVal, "lang"),
					Code:       code,
					RequiresTx: false,
				}
				if v, err := implVal.LookupPath(cue.ParsePath("tx")).Bool(); err == nil {
					impl.RequiresTx = v
				}
				importsVal := implVal.LookupPath(cue.ParsePath("imports"))
				if importsVal.Exists() {
					switch importsVal.IncompleteKind() {
					case cue.ListKind:
						list, _ := importsVal.List()
						for list.Next() {
							s, _ := list.Value().String()
							if strings.TrimSpace(s) != "" {
								impl.Imports = append(impl.Imports, strings.TrimSpace(s))
							}
						}
					default:
						if s, err := importsVal.String(); err == nil && strings.TrimSpace(s) != "" {
							impl.Imports = append(impl.Imports, strings.TrimSpace(s))
						}
					}
				}
				method.Impl = impl
			}
		}

		// Extract flow steps
		flowVal := value.LookupPath(cue.ParsePath("flow"))
		if flowVal.Exists() && flowVal.Kind() == cue.ListKind {
			steps, err := n.parseFlowSteps(flowVal)
			if err != nil {
				return nil, err
			}
			method.Flow = steps

			// Validate flow steps and report warnings
			warnings := validateFlowSteps(opName, svcName, steps, entities)
			for _, w := range warnings {
				n.Warn(Warning{
					Kind:         "flow",
					Code:         w.Code,
					Severity:     w.Severity,
					Message:      w.Message,
					Op:           w.Op,
					Step:         w.Step,
					Action:       w.Action,
					Hint:         w.Hint,
					File:         w.File,
					Line:         w.Line,
					Column:       w.Column,
					CUEPath:      w.CUEPath,
					SuggestedFix: w.SuggestedFix,
				})
			}
		}

		throwsVal := value.LookupPath(cue.ParsePath("throws"))
		if throwsVal.Exists() {
			list, _ := throwsVal.List()
			for list.Next() {
				s, _ := list.Value().String()
				method.Throws = append(method.Throws, strings.TrimSpace(s))
			}
		}

		pubVal := value.LookupPath(cue.ParsePath("publishes"))
		if pubVal.Exists() {
			list, _ := pubVal.List()
			for list.Next() {
				s, _ := list.Value().String()
				eventName := strings.TrimSpace(s)
				if eventName == "" {
					continue
				}
				method.Publishes = append(method.Publishes, eventName)
				found := false
				for _, existing := range svc.Publishes {
					if existing == eventName {
						found = true
						break
					}
				}
				if !found {
					svc.Publishes = append(svc.Publishes, eventName)
				}
				svc.RequiresNats = true
			}
		}

		bcVal := value.LookupPath(cue.ParsePath("broadcasts"))
		if bcVal.Exists() {
			list, _ := bcVal.List()
			for list.Next() {
				s, _ := list.Value().String()
				eventName := strings.TrimSpace(s)
				if eventName != "" {
					method.Broadcasts = append(method.Broadcasts, eventName)
				}
			}
		}

		subVal := value.LookupPath(cue.ParsePath("subscribes"))
		if subVal.Exists() {
			subIter, _ := subVal.Fields()
			for subIter.Next() {
				evtName := strings.TrimSpace(subIter.Selector().String())
				handler, _ := subIter.Value().String()
				handler = strings.TrimSpace(handler)
				svc.Subscribes[evtName] = handler
				svc.RequiresNats = true
			}
		}

		pgVal := value.LookupPath(cue.ParsePath("pagination"))
		if pgVal.Exists() {
			p := &PaginationDef{}
			p.Type = getString(pgVal, "type")
			if p.Type != "" {
				if v, err := pgVal.LookupPath(cue.ParsePath("default_limit")).Int64(); err == nil {
					p.DefaultLimit = int(v)
				}
				if v, err := pgVal.LookupPath(cue.ParsePath("max_limit")).Int64(); err == nil {
					p.MaxLimit = int(v)
				}
				method.Pagination = p
				addPaginationFields(&method)
			}
		}

		// Inferred Pagination: if output contains a list and no explicit pagination, default to offset
		if method.Pagination == nil {
			isList := false
			for _, f := range method.Output.Fields {
				if f.IsList {
					isList = true
					break
				}
			}
			if isList {
				method.Pagination = &PaginationDef{
					Type:         "offset",
					DefaultLimit: 20,
					MaxLimit:     100,
				}
				addPaginationFields(&method)
			}
		}

		svc.Methods = append(svc.Methods, method)
	}

	for _, svc := range serviceMap {
		sort.Slice(svc.Methods, func(i, j int) bool {
			return svc.Methods[i].Name < svc.Methods[j].Name
		})
		services = append(services, *svc)
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services, nil
}

// parseFlowSteps parses flow steps from CUE and auto-completes missing fields
func (n *Normalizer) parseFlowSteps(val cue.Value) ([]FlowStep, error) {
	steps, err := n.rawParseFlowSteps(val)
	if err != nil {
		return nil, err
	}
	return n.autoCompleteFlowSteps(steps), nil
}

// rawParseFlowSteps parses flow steps without auto-completion
func (n *Normalizer) rawParseFlowSteps(val cue.Value) ([]FlowStep, error) {
	var steps []FlowStep

	list, err := val.List()
	if err != nil {
		return nil, err
	}

	for list.Next() {
		stepVal := list.Value()
		action, _ := stepVal.LookupPath(cue.ParsePath("action")).String()
		if action == "" {
			continue
		}

		file := ""
		line := 0
		column := 0
		if pos := stepVal.Pos(); pos.IsValid() {
			file = pos.Filename()
			line = pos.Line()
			column = pos.Column()
			if file != "" {
				if cwd, err := os.Getwd(); err == nil {
					if rel, err := filepath.Rel(cwd, file); err == nil && !strings.HasPrefix(rel, "..") {
						file = rel
					}
				}
			}
		}
		step := FlowStep{
			Action:     action,
			Args:       make(map[string]any),
			File:       file,
			Line:       line,
			Column:     column,
			CUEPath:    stepVal.Path().String(),
			Attributes: parseAttributes(stepVal),
		}

		// Iterate over ALL fields
		it, _ := stepVal.Fields(cue.All())
		for it.Next() {
			label := it.Selector().String()

			// Skip recursion fields and internal definitions
			if label == "action" || label == "then" || label == "else" || label == "do" || strings.HasPrefix(label, "#") {
				continue
			}

			v := it.Value()
			if !v.IsConcrete() && v.Kind() != cue.ListKind {
				continue
			}

			switch v.Kind() {
			case cue.StringKind:
				if s, err := v.String(); err == nil {
					step.Args[label] = s
					if strings.HasPrefix(label, "_") {
						step.Args[strings.TrimPrefix(label, "_")] = s
					}
				}
			case cue.BoolKind:
				if b, err := v.Bool(); err == nil {
					step.Args[label] = b
					if strings.HasPrefix(label, "_") {
						step.Args[strings.TrimPrefix(label, "_")] = b
					}
				}
			case cue.ListKind:
				var p []string
				l, _ := v.List()
				for l.Next() {
					s, err := l.Value().String()
					if err == nil {
						p = append(p, s)
					} else {
						p = append(p, fmt.Sprintf("%v", l.Value()))
					}
				}
				if label == "params" {
					step.Params = p
				} else {
					step.Args[label] = p
					if strings.HasPrefix(label, "_") {
						step.Args[strings.TrimPrefix(label, "_")] = p
					}
				}
			}
		}

		// Double check args/params via explicit lookup if missed in loop
		for _, label := range []string{"args", "params"} {
			if _, ok := step.Args[label]; !ok && label != "params" {
				v := stepVal.LookupPath(cue.ParsePath(label))
				if v.Exists() {
					if v.Kind() == cue.ListKind {
						var p []string
						l, _ := v.List()
						for l.Next() {
							s, _ := l.Value().String()
							if s != "" {
								p = append(p, s)
							}
						}
						step.Args[label] = p
					} else if s, err := v.String(); err == nil && s != "" {
						step.Args[label] = []string{s}
					}
				}
			}
		}

		// Handle recursion for nested steps
		if v := stepVal.LookupPath(cue.ParsePath("then")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_then"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("else")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_else"] = sub
			}
		}
		if v := stepVal.LookupPath(cue.ParsePath("do")); v.Exists() && v.Kind() == cue.ListKind {
			if sub, err := n.parseFlowSteps(v); err == nil {
				step.Args["_do"] = sub
			}
		}

		steps = append(steps, step)
	}

	return steps, nil
}

// autoCompleteFlowSteps injects missing ID/CreatedAt fields before repo.Save for new entities
func (n *Normalizer) autoCompleteFlowSteps(steps []FlowStep) []FlowStep {
	assigned := make(map[string]bool)
	newEntities := make(map[string]bool)

	// First pass: identify new entities and assigned fields
	var scan func([]FlowStep)
	scan = func(steps []FlowStep) {
		for _, s := range steps {
			switch s.Action {
			case "mapping.Map":
				out := fmt.Sprint(s.Args["output"])
				if out == "" {
					out = fmt.Sprint(s.Args["to"])
				}
				if strings.HasPrefix(strings.ToLower(out), "new") {
					newEntities[out] = true
				}
			case "mapping.Assign":
				assigned[fmt.Sprint(s.Args["to"])] = true
			case "tx.Block", "flow.Block":
				if v, ok := s.Args["_do"].([]FlowStep); ok {
					scan(v)
				}
			case "flow.If":
				if v, ok := s.Args["_then"].([]FlowStep); ok {
					scan(v)
				}
				if v, ok := s.Args["_else"].([]FlowStep); ok {
					scan(v)
				}
			case "flow.For":
				if v, ok := s.Args["_do"].([]FlowStep); ok {
					scan(v)
				}
			}
		}
	}
	scan(steps)

	// Second pass: inject missing fields before repo.Save
	var inject func([]FlowStep) []FlowStep
	inject = func(steps []FlowStep) []FlowStep {
		var result []FlowStep
		for _, s := range steps {
			if s.Action == "repo.Save" {
				input := fmt.Sprint(s.Args["input"])
				if newEntities[input] {
					// Inject ID if missing
					if !assigned[input+".ID"] {
						result = append(result, FlowStep{
							Action: "mapping.Assign",
							Args:   map[string]any{"to": input + ".ID", "value": "uuid.NewString()", "generated": "true"},
						})
						assigned[input+".ID"] = true
					}
					// Inject CreatedAt if missing
					if !assigned[input+".CreatedAt"] {
						result = append(result, FlowStep{
							Action: "mapping.Assign",
							Args:   map[string]any{"to": input + ".CreatedAt", "value": "time.Now().UTC().Format(time.RFC3339)", "generated": "true"},
						})
						assigned[input+".CreatedAt"] = true
					}
				}
			}

			// Recurse into nested steps
			if v, ok := s.Args["_do"].([]FlowStep); ok {
				s.Args["_do"] = inject(v)
			}
			if v, ok := s.Args["_then"].([]FlowStep); ok {
				s.Args["_then"] = inject(v)
			}
			if v, ok := s.Args["_else"].([]FlowStep); ok {
				s.Args["_else"] = inject(v)
			}

			result = append(result, s)
		}
		return result
	}

	return inject(steps)
}

// validateFlowSteps checks flow steps for common mistakes and returns warnings
type FlowWarning struct {
	Op           string
	Step         int
	Action       string
	Message      string
	Code         string
	Severity     string
	Hint         string
	File         string
	Line         int
	Column       int
	CUEPath      string
	SuggestedFix []Fix
}

func validateFlowSteps(opName string, svcName string, steps []FlowStep, entities []Entity) []FlowWarning {
	var warnings []FlowWarning
	declaredVars := make(map[string]bool)
	assignedFields := make(map[string]bool)
	newEntities := make(map[string]string)

	entityOwners := make(map[string]string)
	isDTO := make(map[string]bool)
	for _, e := range entities {
		entityOwners[e.Name] = e.Owner
		if dto, ok := e.Metadata["dto"].(bool); ok && dto {
			isDTO[e.Name] = true
		}
	}

	var currentStep FlowStep
	addWarn := func(step int, action, code, message, hint string, file string, line int, column int, fixes ...Fix) {
		warnings = append(warnings, FlowWarning{
			Op:           opName,
			Step:         step,
			Action:       action,
			Message:      message,
			Code:         code,
			Severity:     "error",
			Hint:         hint,
			File:         file,
			Line:         line,
			Column:       column,
			CUEPath:      currentStep.CUEPath,
			SuggestedFix: fixes,
		})
	}

	var validate func(steps []FlowStep, inTx bool, depth int)
	validate = func(steps []FlowStep, inTx bool, depth int) {
		for i := range steps {
			step := &steps[i]
			currentStep = *step
			stepNum := i + 1

			switch step.Action {
			case "repo.Find", "repo.Get", "repo.GetForUpdate", "repo.Save", "repo.Delete", "repo.List":
				source, _ := step.Args["source"].(string)
				if source != "" {
					owner, ok := entityOwners[source]

					if !ok {
						addWarn(stepNum, step.Action, "UNKNOWN_ENTITY",
							fmt.Sprintf("Entity '%s' is not defined in any domain CUE file", source),
							"Define the entity in cue/domain/ or check spelling", step.File, step.Line, step.Column)
					} else if isDTO[source] {
						addWarn(stepNum, step.Action, "DTO_AS_REPO",
							fmt.Sprintf("Entity '%s' is a DTO-only entity and cannot be accessed via repository", source),
							"Remove @dto(only=true) or use a real domain entity", step.File, step.Line, step.Column)
					}

					// Exceptions for basic entities that everyone needs to read
					isShared := strings.EqualFold(source, "Company") || strings.EqualFold(source, "APIKey")

					// Match logic: ignore case and trailing 's' (plural/singular)
					ownerMatch := strings.EqualFold(owner, svcName) ||
						strings.EqualFold(owner+"s", svcName) ||
						strings.EqualFold(svcName+"s", owner)

					// If entity has an owner and it's not THIS service, it's a violation!
					if ok && owner != "" && !isShared && !ownerMatch && !strings.EqualFold(svcName, "admin") && !strings.EqualFold(svcName, "audit") {
						addWarn(stepNum, step.Action, "ARCHITECTURE_VIOLATION",
							fmt.Sprintf("Service '%s' is not allowed to directly access entity '%s' (owned by '%s')", svcName, source, owner),
							fmt.Sprintf("Use events or call %sService", strings.Title(owner)), step.File, step.Line, step.Column)
					}
				}

				// Standard checks ...
				if strings.HasPrefix(step.Action, "repo.Find") || strings.HasPrefix(step.Action, "repo.Get") {
					output, _ := step.Args["output"].(string)
					if output != "" {
						declaredVars[output] = true
					}
				}
				if step.Action == "repo.List" {
					output, _ := step.Args["output"].(string)
					if output != "" {
						declaredVars[output] = true
					}
				}
				if step.Action == "repo.GetForUpdate" && !inTx {
					addWarn(stepNum, step.Action, "TX_REQUIRED", "repo.GetForUpdate outside tx.Block", "{action: \"tx.Block\", do: [ ... ]}", step.File, step.Line, step.Column)
				}

			case "mapping.Map":
				output, _ := step.Args["output"].(string)
				if output == "" {
					output, _ = step.Args["to"].(string)
				}
				entity, _ := step.Args["entity"].(string)
				if output != "" && strings.HasPrefix(strings.ToLower(output), "new") && entity == "" {
					addWarn(stepNum, step.Action, "MISSING_ENTITY", fmt.Sprintf("mapping.Map '%s' missing 'entity'", output), "{action: \"mapping.Map\", output: \""+output+"\", entity: \"Entity\"}", step.File, step.Line, step.Column)
				}
				if output != "" && entity != "" {
					declaredVars[output] = true
					newEntities[output] = entity
				}

			case "mapping.Assign":
				to, _ := step.Args["to"].(string)
				value, _ := step.Args["value"].(string)
				if to == "" {
					addWarn(stepNum, step.Action, "MISSING_TO", "mapping.Assign missing 'to'", "{action: \"mapping.Assign\", to: \"x.Field\", value: \"...\"}", step.File, step.Line, step.Column)
				}
				if value == "" {
					addWarn(stepNum, step.Action, "MISSING_VALUE", "mapping.Assign missing 'value'", "{action: \"mapping.Assign\", to: \"x.Field\", value: \"...\"}", step.File, step.Line, step.Column)
				}
				assignedFields[to] = true

				// NEW: Validate Go Syntax
				if errStr := validateGoSnippet(value, step.File, step.Line, step.Column); errStr != "" {
					addWarn(stepNum, step.Action, "GO_SYNTAX_ERROR", errStr, "Check your Go code syntax inside the CUE string.", step.File, step.Line, step.Column)
				}

				// Check for unquoted status strings
				statusValues := map[string]bool{"draft": true, "active": true, "pending": true, "published": true, "closed": true, "approved": true, "rejected": true, "cancelled": true}
				if value != "" && !strings.Contains(value, "\"") && !strings.Contains(value, ".") && !strings.Contains(value, "(") {
					if statusValues[strings.ToLower(value)] {
						addWarn(stepNum, step.Action, "NEEDS_QUOTES", fmt.Sprintf("mapping.Assign '%s' needs quotes: \"\\\"%s\\\"\"", value, value), "{action: \"mapping.Assign\", to: \"x.Status\", value: \"\\\""+value+"\\\"\"}", step.File, step.Line, step.Column, Fix{
							Kind: "replace",
							Text: "\"" + value + "\"",
						})
					}
				}

			case "event.Publish":
				name, _ := step.Args["name"].(string)
				payload, _ := step.Args["payload"].(string)
				if name == "" {
					addWarn(stepNum, step.Action, "MISSING_NAME", "event.Publish missing 'name'", "{action: \"event.Publish\", name: \"EventName\", payload: \"domain.EventName{...}\"}", step.File, step.Line, step.Column)
				}
				if payload != "" && !strings.HasPrefix(payload, "domain.") {
					addWarn(stepNum, step.Action, "PAYLOAD_NOT_DOMAIN", fmt.Sprintf("event.Publish payload should use domain.%s{...}", name), "{action: \"event.Publish\", name: \""+name+"\", payload: \"domain."+name+"{...}\"}", step.File, step.Line, step.Column)
				}

			case "logic.Check":
				cond, _ := step.Args["condition"].(string)
				if cond == "" {
					addWarn(stepNum, step.Action, "MISSING_CONDITION", "logic.Check missing 'condition'", "{action: \"logic.Check\", condition: \"x > 0\", throw: \"ERROR_CODE\"}", step.File, step.Line, step.Column)
				} else {
					if errStr := validateGoSnippet(cond, step.File, step.Line, step.Column); errStr != "" {
						addWarn(stepNum, step.Action, "GO_SYNTAX_ERROR", errStr, "Check your Go code syntax inside the CUE string.", step.File, step.Line, step.Column)
					}
				}
				if step.Args["throw"] == nil || step.Args["throw"] == "" {
					addWarn(stepNum, step.Action, "MISSING_THROW", "logic.Check missing 'throw'", "{action: \"logic.Check\", condition: \"x > 0\", throw: \"ERROR_CODE\"}", step.File, step.Line, step.Column)
				}

			case "logic.Call":
				if step.Args["func"] == nil || step.Args["func"] == "" {
					addWarn(stepNum, step.Action, "MISSING_FUNC", "logic.Call missing 'func'", "{action: \"logic.Call\", func: \"DoThing\", args: [\"a\", \"b\"]}", step.File, step.Line, step.Column)
				}
				// Normalize args to []string for templates
				if args, ok := step.Args["args"]; ok {
					switch v := args.(type) {
					case string:
						step.Args["args"] = []string{v}
					case []any:
						var ss []string
						for _, x := range v {
							ss = append(ss, fmt.Sprint(x))
						}
						step.Args["args"] = ss
					}
				} else {
					step.Args["args"] = []string{}
				}

			case "list.Append":
				if step.Args["to"] == nil || step.Args["to"] == "" {
					addWarn(stepNum, step.Action, "MISSING_TO", "list.Append missing 'to'", "{action: \"list.Append\", to: \"resp.Items\", item: \"x\"}", step.File, step.Line, step.Column)
				}
				if step.Args["item"] == nil || step.Args["item"] == "" {
					addWarn(stepNum, step.Action, "MISSING_ITEM", "list.Append missing 'item'", "{action: \"list.Append\", to: \"resp.Items\", item: \"x\"}", step.File, step.Line, step.Column)
				}

			case "fsm.Transition":
				if step.Args["entity"] == nil || step.Args["entity"] == "" {
					addWarn(stepNum, step.Action, "MISSING_ENTITY", "fsm.Transition missing 'entity'", "{action: \"fsm.Transition\", entity: \"order\", to: \"confirmed\"}", step.File, step.Line, step.Column)
				}
				if step.Args["to"] == nil || step.Args["to"] == "" {
					addWarn(stepNum, step.Action, "MISSING_TO", "fsm.Transition missing 'to'", "{action: \"fsm.Transition\", entity: \"order\", to: \"confirmed\"}", step.File, step.Line, step.Column)
				}

			case "tx.Block", "flow.Block":
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					if len(subSteps) == 0 {
						addWarn(stepNum, step.Action, "EMPTY_DO", fmt.Sprintf("%s has empty 'do'", step.Action), "{action: \""+step.Action+"\", do: [ ... ]}", step.File, step.Line, step.Column)
					}
					validate(subSteps, step.Action == "tx.Block", depth+1)
				} else {
					addWarn(stepNum, step.Action, "MISSING_DO", fmt.Sprintf("%s missing 'do'", step.Action), "{action: \""+step.Action+"\", do: [ ... ]}", step.File, step.Line, step.Column)
				}

			case "flow.If":
				if step.Args["condition"] == nil || step.Args["condition"] == "" {
					addWarn(stepNum, step.Action, "MISSING_CONDITION", "flow.If missing 'condition'", "{action: \"flow.If\", condition: \"x == y\", then: [ ... ]}", step.File, step.Line, step.Column)
				}
				if subSteps, ok := step.Args["_then"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				} else {
					addWarn(stepNum, step.Action, "MISSING_THEN", "flow.If missing 'then'", "{action: \"flow.If\", condition: \"x == y\", then: [ ... ]}", step.File, step.Line, step.Column)
				}
				if subSteps, ok := step.Args["_else"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				}

			case "flow.For":
				if step.Args["each"] == nil || step.Args["each"] == "" {
					addWarn(stepNum, step.Action, "MISSING_EACH", "flow.For missing 'each'", "{action: \"flow.For\", each: \"items\", as: \"item\", do: [ ... ]}", step.File, step.Line, step.Column)
				}
				if step.Args["as"] == nil || step.Args["as"] == "" {
					addWarn(stepNum, step.Action, "MISSING_AS", "flow.For missing 'as'", "{action: \"flow.For\", each: \"items\", as: \"item\", do: [ ... ]}", step.File, step.Line, step.Column)
				}
				if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
					validate(subSteps, inTx, depth+1)
				} else {
					addWarn(stepNum, step.Action, "MISSING_DO", "flow.For missing 'do'", "{action: \"flow.For\", each: \"items\", as: \"item\", do: [ ... ]}", step.File, step.Line, step.Column)
				}

			default:
				if step.Action != "" && !strings.HasPrefix(step.Action, "repo.") && !strings.HasPrefix(step.Action, "mapping.") &&
					!strings.HasPrefix(step.Action, "logic.") && !strings.HasPrefix(step.Action, "event.") &&
					!strings.HasPrefix(step.Action, "fsm.") && !strings.HasPrefix(step.Action, "flow.") &&
					!strings.HasPrefix(step.Action, "tx.") && !strings.HasPrefix(step.Action, "list.") {
					addWarn(stepNum, step.Action, "UNKNOWN_ACTION", fmt.Sprintf("unknown action '%s'", step.Action), "{action: \"repo.Find\" | \"mapping.Assign\" | \"flow.If\" ...}", step.File, step.Line, step.Column)
				}
			}
		}
	}

	validate(steps, false, 0)
	return warnings
}

func (n *Normalizer) parseService(name string, val cue.Value) (Service, error) {
	svcDescription, _ := val.LookupPath(cue.ParsePath("description")).String()
	svc := Service{
		Name:        name,
		Description: svcDescription,
		Subscribes:  make(map[string]string),
	}

	iter, err := val.Fields(cue.All())
	if err != nil {
		return svc, err
	}

	for iter.Next() {
		methodName := cleanName(iter.Selector().String())
		methodVal := iter.Value()

		if strings.HasPrefix(methodName, "$") || methodName == "publishes" || methodName == "subscribes" {
			continue
		}

		mDescription, _ := methodVal.LookupPath(cue.ParsePath("description")).String()
		method := Method{
			Name:        methodName,
			Description: mDescription,
		}

		inVal := methodVal.LookupPath(cue.ParsePath("in"))
		if inVal.Exists() {
			ent, err := n.parseEntity(methodName+"Request", inVal)
			if err != nil {
				return svc, fmt.Errorf("failed to parse input for %s.%s: %w", name, methodName, err)
			}
			method.Input = ent
		}

		outVal := methodVal.LookupPath(cue.ParsePath("out"))
		if outVal.Exists() {
			ent, err := n.parseEntity(methodName+"Response", outVal)
			if err != nil {
				return svc, fmt.Errorf("failed to parse output for %s.%s: %w", name, methodName, err)
			}
			method.Output = ent
		}

		cacheAttr := methodVal.Attribute("cache")
		if cacheAttr.Err() == nil {
			if val, found, _ := cacheAttr.Lookup(0, "ttl"); found {
				method.CacheTTL = val
				svc.RequiresRedis = true
			}
		}

		srcVal := methodVal.LookupPath(cue.ParsePath("sources"))
		if srcVal.Exists() {
			srcIter, _ := srcVal.Fields()
			for srcIter.Next() {
				sName := srcIter.Selector().String()
				sVal := srcIter.Value()

				kind := getString(sVal, "kind")
				source := Source{
					Name:       sName,
					Kind:       kind,
					Entity:     getString(sVal, "entity"),
					Collection: getString(sVal, "collection"),
					By:         make(map[string]string),
					Filter:     make(map[string]string),
					Metadata:   make(map[string]any),
				}

				switch kind {
				case "sql":
					svc.RequiresSQL = true
				case "mongo":
					svc.RequiresMongo = true
				case "redis":
					svc.RequiresRedis = true
				case "s3":
					svc.RequiresS3 = true
				}

				byVal := sVal.LookupPath(cue.ParsePath("by"))
				if byVal.Exists() {
					bit, _ := byVal.Fields()
					for bit.Next() {
						v, _ := bit.Value().String()
						source.By[bit.Selector().String()] = strings.Trim(v, "")
					}
				}

				filterVal := sVal.LookupPath(cue.ParsePath("filter"))
				if filterVal.Exists() {
					fit, _ := filterVal.Fields()
					for fit.Next() {
						v, _ := fit.Value().String()
						source.Filter[fit.Selector().String()] = strings.Trim(v, "")
					}
				}

				method.Sources = append(method.Sources, source)
			}
		}

		var implVal cue.Value
		if iv := methodVal.LookupPath(cue.ParsePath("impls")); iv.Exists() {
			if gv := iv.LookupPath(cue.ParsePath("go")); gv.Exists() {
				implVal = gv
			}
		}
		if !implVal.Exists() {
			implVal = methodVal.LookupPath(cue.ParsePath("_impl"))
		}
		if implVal.Exists() && implVal.IncompleteKind() == cue.BottomKind {
			implVal = cue.Value{}
		}
		if !implVal.Exists() {
			implVal = methodVal.LookupPath(cue.ParsePath("impl"))
		}

		if implVal.Exists() {
			codeVal := implVal.LookupPath(cue.ParsePath("code"))
			code, _ := codeVal.String()

			if code != "" {
				impl := &MethodImpl{
					Lang:       getString(implVal, "lang"),
					Code:       code,
					RequiresTx: false,
				}
				if v, err := implVal.LookupPath(cue.ParsePath("tx")).Bool(); err == nil {
					impl.RequiresTx = v
				}
				importsVal := implVal.LookupPath(cue.ParsePath("imports"))
				if importsVal.Exists() {
					switch importsVal.IncompleteKind() {
					case cue.ListKind:
						list, _ := importsVal.List()
						for list.Next() {
							s, _ := list.Value().String()
							if strings.TrimSpace(s) != "" {
								impl.Imports = append(impl.Imports, strings.TrimSpace(s))
							}
						}
					default:
						if s, err := importsVal.String(); err == nil && strings.TrimSpace(s) != "" {
							impl.Imports = append(impl.Imports, strings.TrimSpace(s))
						}
					}
				}
				method.Impl = impl
			}
		}

		flowVal := methodVal.LookupPath(cue.ParsePath("flow"))
		if flowVal.Exists() && flowVal.Kind() == cue.ListKind {
			steps, err := n.parseFlowSteps(flowVal)
			if err != nil {
				return svc, err
			}
			method.Flow = steps
		}

		svc.Methods = append(svc.Methods, method)
	}

	pubVal := val.LookupPath(cue.ParsePath("publishes"))
	if pubVal.Exists() {
		list, _ := pubVal.List()
		for list.Next() {
			s, _ := list.Value().String()
			svc.Publishes = append(svc.Publishes, strings.Trim(s, ""))
			svc.RequiresNats = true
		}
	}

	subVal := val.LookupPath(cue.ParsePath("subscribes"))
	if subVal.Exists() {
		subIter, _ := subVal.Fields()
		for subIter.Next() {
			evtName := subIter.Selector().String()
			handler, _ := subIter.Value().String()
			svc.Subscribes[evtName] = strings.Trim(handler, "")
			svc.RequiresNats = true

			method := Method{
				Name:  strings.Trim(handler, ""),
				Input: Entity{Name: evtName},
			}
			svc.Methods = append(svc.Methods, method)
		}
	}
	return svc, nil
}

func (n *Normalizer) ExtractEndpoints(val cue.Value) ([]Endpoint, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}
	var endpoints []Endpoint

	httpVal := val.LookupPath(cue.ParsePath("HTTP"))
	if !httpVal.Exists() {
		return nil, nil
	}

	// Extract default_rate_limit if defined
	var defaultRateLimit *RateLimitDef
	defaultRLVal := httpVal.LookupPath(cue.ParsePath("default_rate_limit"))
	if defaultRLVal.Exists() {
		defaultRateLimit = &RateLimitDef{}
		if v, err := defaultRLVal.LookupPath(cue.ParsePath("rps")).Int64(); err == nil {
			defaultRateLimit.RPS = int(v)
		}
		if v, err := defaultRLVal.LookupPath(cue.ParsePath("burst")).Int64(); err == nil {
			defaultRateLimit.Burst = int(v)
		}
	}

	// Extract default_timeout if defined
	var defaultTimeout string
	if v, err := httpVal.LookupPath(cue.ParsePath("default_timeout")).String(); err == nil {
		defaultTimeout = v
	}

	// Extract default_max_body_size if defined
	defaultMaxBodySize := parseSize("1mb") // standard default
	if v, err := httpVal.LookupPath(cue.ParsePath("default_max_body_size")).String(); err == nil {
		defaultMaxBodySize = parseSize(v)
	}

	type opInfo struct {
		name  string
		value cue.Value
	}
	ops := make(map[string]opInfo)
	iter, err := val.Fields(cue.All())
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		label := iter.Selector().String()
		if strings.HasPrefix(label, "#") || label == "HTTP" {
			continue
		}
		opVal := iter.Value()
		if getString(opVal, "service") == "" {
			continue
		}
		name := cleanName(label)
		ops[name] = opInfo{name: name, value: opVal}
	}

	apiIter, _ := httpVal.Fields(cue.All())
	for apiIter.Next() {
		epName := cleanName(apiIter.Selector().String())
		// Skip config fields - they're not endpoints
		if epName == "default_rate_limit" || epName == "default_timeout" || epName == "default_max_body_size" {
			continue
		}
		epVal := apiIter.Value()

		opInfo, ok := ops[epName]
		if !ok {
			return nil, fmt.Errorf("HTTP endpoint %s has no matching operation", epName)
		}

		svcName := normalizeServiceName(getString(opInfo.value, "service"))
		if svcName == "" {
			return nil, fmt.Errorf("missing service for operation %s", epName)
		}

		method := getString(epVal, "method")
		ep := Endpoint{
			Method:      method,
			Path:        getString(epVal, "path"),
			ServiceName: svcName,
			RPC:         epName,
			Description: getString(epVal, "description"),
			RoomParam:   getString(epVal, "room"),
			AuthType:    getString(epVal, "auth.type"),
			Permission:  getString(epVal, "auth.permission"),
			AuthCheck:   getString(epVal, "auth.check"),
			CacheTTL:    getString(epVal, "cache.ttl"),
			View:        getString(epVal, "view"),
			Source:      formatPos(epVal),
		}

		// Extract testHints from operation or HTTP definition
		hintsVal := opInfo.value.LookupPath(cue.ParsePath("testHints"))
		if !hintsVal.Exists() {
			hintsVal = epVal.LookupPath(cue.ParsePath("testHints"))
		}
		if hintsVal.Exists() {
			ep.TestHints = &TestHints{
				HappyPath: getString(hintsVal, "happyPath"),
			}
			errVal := hintsVal.LookupPath(cue.ParsePath("errorCases"))
			if errVal.Exists() {
				it, _ := errVal.List()
				for it.Next() {
					s, _ := it.Value().String()
					ep.TestHints.ErrorCases = append(ep.TestHints.ErrorCases, s)
				}
			}
		}

		tagsVal := epVal.LookupPath(cue.ParsePath("cache.tags"))
		if tagsVal.Exists() {
			it, _ := tagsVal.List()
			for it.Next() {
				s, _ := it.Value().String()
				ep.CacheTags = append(ep.CacheTags, s)
			}
		}

		invVal := epVal.LookupPath(cue.ParsePath("invalidate"))
		if invVal.Exists() {
			it, _ := invVal.List()
			for it.Next() {
				s, _ := it.Value().String()
				ep.Invalidate = append(ep.Invalidate, s)
			}
		}

		if v, err := epVal.LookupPath(cue.ParsePath("optimistic_update")).String(); err == nil {
			ep.OptimisticUpdate = v
		}

		// Smart Defaults: Auto-invalidate lists on mutations
		if ep.Method != "GET" && ep.Method != "WS" && len(ep.Invalidate) == 0 {
			// Find all GET endpoints in the same service that look like lists
			for _, other := range ops {
				if getString(other.value, "service") == getString(opInfo.value, "service") {
					// If it starts with List or AdminList, it's a candidate
					if strings.HasPrefix(other.name, "List") || strings.HasPrefix(other.name, "AdminList") {
						ep.Invalidate = append(ep.Invalidate, other.name)
					}
				}
			}
		}
		sort.Strings(ep.Invalidate)

		msgsVal := epVal.LookupPath(cue.ParsePath("messages"))
		if msgsVal.Exists() {
			list, _ := msgsVal.List()
			for list.Next() {
				s, _ := list.Value().String()
				ep.Messages = append(ep.Messages, strings.TrimSpace(s))
			}
		}

		// Extract pagination from operation if exists
		pgVal := opInfo.value.LookupPath(cue.ParsePath("pagination"))
		if pgVal.Exists() {
			p := &PaginationDef{}
			p.Type = getString(pgVal, "type")
			if p.Type != "" {
				if v, err := pgVal.LookupPath(cue.ParsePath("default_limit")).Int64(); err == nil {
					p.DefaultLimit = int(v)
				}
				if v, err := pgVal.LookupPath(cue.ParsePath("max_limit")).Int64(); err == nil {
					p.MaxLimit = int(v)
				}
				ep.Pagination = p
			}
		}

		// Inferred Pagination for Endpoints
		if ep.Pagination == nil {
			outVal := opInfo.value.LookupPath(cue.ParsePath("output"))
			if !outVal.Exists() {
				outVal = opInfo.value.LookupPath(cue.ParsePath("out"))
			}
			if outVal.Exists() {
				ent, err := n.parseEntity(epName+"Response", outVal)
				if err == nil {
					isList := false
					for _, f := range ent.Fields {
						if f.IsList {
							isList = true
							break
						}
					}
					if isList {
						ep.Pagination = &PaginationDef{
							Type:         "offset",
							DefaultLimit: 20,
							MaxLimit:     100,
						}
					}
				}
			}
		}

		if ep.Permission == "" {
			ep.Permission = getString(epVal, "auth.action")
		}

		rolesVal := epVal.LookupPath(cue.ParsePath("auth.roles"))
		if rolesVal.Exists() {
			list, _ := rolesVal.List()
			for list.Next() {
				s, _ := list.Value().String()
				ep.AuthRoles = append(ep.AuthRoles, strings.TrimSpace(s))
			}
		}

		injectVal := epVal.LookupPath(cue.ParsePath("auth.inject"))
		if injectVal.Exists() {
			switch injectVal.IncompleteKind() {
			case cue.ListKind:
				list, _ := injectVal.List()
				for list.Next() {
					s, _ := list.Value().String()
					if strings.TrimSpace(s) != "" {
						ep.AuthInject = append(ep.AuthInject, strings.TrimSpace(s))
					}
				}
			default:
				if s, err := injectVal.String(); err == nil && strings.TrimSpace(s) != "" {
					ep.AuthInject = append(ep.AuthInject, strings.TrimSpace(s))
				}
			}
		}

		if val, err := epVal.LookupPath(cue.ParsePath("idempotency")).Bool(); err == nil {
			ep.Idempotency = val
		}

		for _, attr := range epVal.Attributes(cue.ValueAttr) {
			switch attr.Name() {
			case "idempotent":
				ep.Idempotency = true
			case "dedupeKey":
				if s, found, _ := attr.Lookup(0, ""); found {
					ep.DedupeKey = s
				}
			}
		}

		rlVal := epVal.LookupPath(cue.ParsePath("rate_limit"))
		if rlVal.Exists() {
			rl := &RateLimitDef{}
			if v, err := rlVal.LookupPath(cue.ParsePath("rps")).Int64(); err == nil {
				rl.RPS = int(v)
			}
			if v, err := rlVal.LookupPath(cue.ParsePath("burst")).Int64(); err == nil {
				rl.Burst = int(v)
			}
			if rl.RPS > 0 || rl.Burst > 0 {
				ep.RateLimit = rl
			}
		}

		// Apply default rate limit if endpoint doesn't have explicit one
		if ep.RateLimit == nil && defaultRateLimit != nil {
			ep.RateLimit = defaultRateLimit
		}

		// Parse timeout
		if v, err := epVal.LookupPath(cue.ParsePath("timeout")).String(); err == nil {
			ep.Timeout = v
		}
		// Apply default timeout if endpoint doesn't have explicit one
		if ep.Timeout == "" && defaultTimeout != "" {
			ep.Timeout = defaultTimeout
		}

		// Parse max body size
		if v, err := epVal.LookupPath(cue.ParsePath("max_body_size")).String(); err == nil {
			ep.MaxBodySize = parseSize(v)
		}
		// Apply default if not set
		if ep.MaxBodySize == 0 {
			ep.MaxBodySize = defaultMaxBodySize
		}

		cbVal := epVal.LookupPath(cue.ParsePath("circuit_breaker"))
		if cbVal.Exists() {
			cb := &CircuitBreakerDef{Threshold: 5, Timeout: "30s", HalfOpenMax: 3}
			if v, err := cbVal.LookupPath(cue.ParsePath("threshold")).Int64(); err == nil {
				cb.Threshold = int(v)
			}
			if v, err := cbVal.LookupPath(cue.ParsePath("timeout")).String(); err == nil {
				cb.Timeout = v
			}
			if v, err := cbVal.LookupPath(cue.ParsePath("half_open_max")).Int64(); err == nil {
				cb.HalfOpenMax = int(v)
			}
			ep.CircuitBreaker = cb
		}

		msgVal := epVal.LookupPath(cue.ParsePath("messages"))
		if msgVal.Exists() {
			switch msgVal.IncompleteKind() {
			case cue.ListKind:
				list, _ := msgVal.List()
				for list.Next() {
					s, _ := list.Value().String()
					ep.Messages = append(ep.Messages, strings.TrimSpace(s))
				}
			case cue.StructKind:
				msgIter, _ := msgVal.Fields()
				for msgIter.Next() {
					ep.Messages = append(ep.Messages, strings.TrimSpace(msgIter.Selector().String()))
				}
			}
		}

		pathInfo := ""
		if p := epVal.Path(); p.String() != "" {
			pathInfo = fmt.Sprintf(" (%s)", p.String())
		}
		if ep.Method == "" || ep.Path == "" || ep.ServiceName == "" {
			return nil, fmt.Errorf("invalid endpoint %s%s: method/path/service are required", epName, pathInfo)
		}
		endpoints = append(endpoints, ep)
	}

	return endpoints, nil
}

func addPaginationFields(method *Method) {
	if method == nil || method.Pagination == nil {
		return
	}
	exists := func(name string) bool {
		for _, f := range method.Input.Fields {
			if f.Name == name {
				return true
			}
		}
		return false
	}
	switch method.Pagination.Type {
	case "offset":
		if !exists("limit") {
			method.Input.Fields = append(method.Input.Fields, Field{Name: "limit", Type: "int", IsOptional: true})
		}
		if !exists("offset") {
			method.Input.Fields = append(method.Input.Fields, Field{Name: "offset", Type: "int", IsOptional: true})
		}
	case "cursor":
		if !exists("cursor") {
			method.Input.Fields = append(method.Input.Fields, Field{Name: "cursor", Type: "string", IsOptional: true})
		}
		if !exists("limit") {
			method.Input.Fields = append(method.Input.Fields, Field{Name: "limit", Type: "int", IsOptional: true})
		}
	}
}

func validateGoSnippet(code string, file string, line int, col int) string {
	if code == "" || strings.Contains(code, "{{") {
		return "" // Skip templates for now
	}
	// Wrap code in a function block
	wrapped := fmt.Sprintf("package dummy\nfunc _() { _ = %s }", code)
	if strings.Contains(code, ";") || strings.Contains(code, "for ") || strings.Contains(code, "if ") {
		wrapped = fmt.Sprintf("package dummy\nfunc _() {\n%s\n}", code)
	}

	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "", wrapped, 0)
	if err != nil {
		return fmt.Sprintf("Invalid Go syntax: %v", err)
	}
	return ""
}
