package normalizer

import (
	"fmt"
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

	// Index repositories for impl_steps validation
	n.RepoNames = make(map[string]struct{})
	for _, e := range entities {
		n.RepoNames[e.Name+"Repository"] = struct{}{}
	}
	// EventNames may already be filled in ExtractEvents; ensure map exists
	if n.EventNames == nil {
		n.EventNames = make(map[string]struct{})
	}

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
		method.PrimaryOperationKind = parseOperationKind(value)
		method.Capabilities = parseCapabilities(value)
		method.SideEffects = parseSideEffects(value)
		method.ManualRequired = parseManualRequired(value)
		if streamVal := value.LookupPath(cue.MakePath(cue.Str("stream"))); streamVal.Exists() {
			if b, err := streamVal.Bool(); err == nil {
				method.IsStreaming = b
			}
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
				for _, diag := range validateNamedReturnImplCode(svcName, opName, method, codeVal) {
					n.Warn(diag)
				}
				for _, diag := range validateImplAntiPatterns(svcName, opName, method, codeVal) {
					n.Warn(diag)
				}
			}

			// Typed impl steps
			if stepsVal := implVal.LookupPath(cue.ParsePath("impl_steps")); stepsVal.Exists() {
				steps, err := n.parseImplSteps(stepsVal)
				if err != nil {
					return nil, fmt.Errorf("parse impl_steps for %s.%s: %w", svcName, opName, err)
				}
				method.ImplSteps = steps
			}
		}

		// Extract flow steps
		var steps []FlowStep
		var hasFlow bool
		flowVal := value.LookupPath(cue.ParsePath("flow"))
		if flowVal.Exists() && flowVal.Kind() == cue.ListKind {
			steps, err = n.parseFlowSteps(flowVal)
			if err != nil {
				return nil, err
			}
			hasFlow = true
		} else if flowfnVal := value.LookupPath(cue.ParsePath("flowfn")); flowfnVal.Exists() && flowfnVal.IncompleteKind() == cue.StringKind {
			raw, err := flowfnVal.String()
			if err != nil {
				return nil, fmt.Errorf("parse flowfn for %s.%s: %w", svcName, opName, err)
			}
			steps, err = n.parseFlowFn(raw)
			if err != nil {
				return nil, fmt.Errorf("parse flowfn for %s.%s: %w", svcName, opName, err)
			}
			hasFlow = true
		}
		if hasFlow {
			method.Flow = steps
			effects := DeriveOperationEffects(steps)
			method.Effects = make([]string, 0, len(effects))
			for _, kind := range effects {
				method.Effects = append(method.Effects, string(kind))
			}
			if flowUsesObjectStorage(steps) {
				svc.RequiresS3 = true
			}
			flowHasAction := func(items []FlowStep, action string) bool {
				var scan func([]FlowStep) bool
				scan = func(nodes []FlowStep) bool {
					for _, node := range nodes {
						if node.Action == action {
							return true
						}
						for _, key := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
							if nested, ok := node.Args[key].([]FlowStep); ok && scan(nested) {
								return true
							}
						}
						if cases, ok := node.Args["_cases"].(map[string][]FlowStep); ok {
							for _, branch := range cases {
								if scan(branch) {
									return true
								}
							}
						}
						if branches, ok := node.Args["_branches"].(map[string][]FlowStep); ok {
							for _, branch := range branches {
								if scan(branch) {
									return true
								}
							}
						}
					}
					return false
				}
				return scan(items)
			}
			if !method.IsStreaming && flowHasAction(steps, "openai.Stream") {
				n.Warn(Warning{
					Kind:     "flow",
					Code:     "STREAM_ACTION_REQUIRES_STREAM_METHOD",
					Severity: "error",
					Message:  fmt.Sprintf("operation '%s' uses openai.Stream but stream: true is not set", opName),
					Hint:     "Set stream: true on operation or replace openai.Stream with openai.Chat",
					File:     method.Source,
					CUEPath:  value.Path().String(),
				})
			}

			// Validate flow steps and report warnings
			warnings := validateFlowSteps(opName, svcName, steps, entities, svc.Uses, n.Policies, n.ArchitectureMode, n.ArchitectureAllowCross)
			for _, w := range warnings {
				canAutoApply := false
				for _, fx := range w.SuggestedFix {
					op := strings.ToLower(strings.TrimSpace(fx.Op))
					if op == "" {
						op = strings.ToLower(strings.TrimSpace(fx.Kind))
					}
					if op == "merge" || op == "replace" || op == "insert" || op == "delete" || op == "create" {
						canAutoApply = true
						break
					}
				}
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
					CanAutoApply: canAutoApply,
					SuggestedFix: w.SuggestedFix,
				})
			}
		}
		if implVal.Exists() {
			codeVal := implVal.LookupPath(cue.ParsePath("code"))
			bypass, _ := implVal.LookupPath(cue.ParsePath("flowFirstBypass")).Bool()
			bypassReasonVal := implVal.LookupPath(cue.ParsePath("flowFirstBypassReason"))
			bypassReason := getString(implVal, "flowFirstBypassReason")
			for _, diag := range validateFlowFirstImplCode(svcName, opName, method, codeVal, bypassReasonVal, bypass, bypassReason) {
				n.Warn(diag)
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
