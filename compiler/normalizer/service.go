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
		flowVal := value.LookupPath(cue.ParsePath("flow"))
		if flowVal.Exists() && flowVal.Kind() == cue.ListKind {
			steps, err := n.parseFlowSteps(flowVal)
			if err != nil {
				return nil, err
			}
			method.Flow = steps
			if flowUsesObjectStorage(steps) {
				svc.RequiresS3 = true
			}

			// Validate flow steps and report warnings
			warnings := validateFlowSteps(opName, svcName, steps, entities, svc.Uses, n.Policies, n.ArchitectureMode, n.ArchitectureAllowCross)
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

// parseFlowSteps parses flow steps from CUE and auto-completes missing fields
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
				for _, diag := range validateNamedReturnImplCode(name, methodName, method, codeVal) {
					n.Warn(diag)
				}
				for _, diag := range validateImplAntiPatterns(name, methodName, method, codeVal) {
					n.Warn(diag)
				}
			}
		}

		flowVal := methodVal.LookupPath(cue.ParsePath("flow"))
		if flowVal.Exists() && flowVal.Kind() == cue.ListKind {
			steps, err := n.parseFlowSteps(flowVal)
			if err != nil {
				return svc, err
			}
			method.Flow = steps
			if flowUsesObjectStorage(steps) {
				svc.RequiresS3 = true
			}
		}
		if implVal.Exists() {
			codeVal := implVal.LookupPath(cue.ParsePath("code"))
			bypass, _ := implVal.LookupPath(cue.ParsePath("flowFirstBypass")).Bool()
			bypassReasonVal := implVal.LookupPath(cue.ParsePath("flowFirstBypassReason"))
			bypassReason := getString(implVal, "flowFirstBypassReason")
			for _, diag := range validateFlowFirstImplCode(name, methodName, method, codeVal, bypassReasonVal, bypass, bypassReason) {
				n.Warn(diag)
			}
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
		if v, err := defaultRLVal.LookupPath(cue.ParsePath("window")).String(); err == nil {
			defaultRateLimit.Window = v
		}
		if v, err := defaultRLVal.LookupPath(cue.ParsePath("limit")).Int64(); err == nil {
			defaultRateLimit.WindowLimit = int(v)
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
		// Intelligent RBAC: extract from @rbac attributes
		for _, attr := range opInfo.value.Attributes(cue.ValueAttr) {
			if attr.Name() == "rbac" {
				val := attr.Contents()
				// Упрощенный парсинг rule=... или role=...
				parts := strings.Split(val, ",")
				for _, p := range parts {
					kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
					if len(kv) == 2 {
						k := kv[0]
						v := strings.Trim(kv[1], "\"")
						if k == "role" {
							ep.AuthRoles = append(ep.AuthRoles, v)
							if ep.AuthType == "" {
								ep.AuthType = "jwt"
							}
						} else if k == "permission" {
							ep.Permission = v
							if ep.AuthType == "" {
								ep.AuthType = "jwt"
							}
						}
					}
				}
			}
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

		// Smart Defaults: Auto-invalidate related list on mutations.
		// Only invalidate the list endpoint(s) whose entity matches this mutation's entity.
		if ep.Method != "GET" && ep.Method != "WS" && len(ep.Invalidate) == 0 {
			mutationEntity := strings.ToLower(rpcEntityBase(ep.RPC))
			svc := getString(opInfo.value, "service")
			for _, other := range ops {
				if getString(other.value, "service") != svc {
					continue
				}
				if !strings.HasPrefix(other.name, "List") && !strings.HasPrefix(other.name, "AdminList") {
					continue
				}
				listEntity := strings.ToLower(rpcEntityBase(other.name))
				if mutationEntity != "" && listEntity != "" &&
					strings.HasPrefix(listEntity, mutationEntity) {
					ep.Invalidate = append(ep.Invalidate, other.name)
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

		// Read auth.scope or auth.scopes — required API key scopes for this endpoint
		scopeVal := epVal.LookupPath(cue.ParsePath("auth.scope"))
		if scopeVal.Exists() {
			switch scopeVal.IncompleteKind() {
			case cue.ListKind:
				list, _ := scopeVal.List()
				for list.Next() {
					s, _ := list.Value().String()
					if strings.TrimSpace(s) != "" {
						ep.RequiredScopes = append(ep.RequiredScopes, strings.TrimSpace(s))
					}
				}
			default:
				if s, err := scopeVal.String(); err == nil && strings.TrimSpace(s) != "" {
					ep.RequiredScopes = append(ep.RequiredScopes, strings.TrimSpace(s))
				}
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
			if v, err := rlVal.LookupPath(cue.ParsePath("window")).String(); err == nil {
				rl.Window = v
			}
			if v, err := rlVal.LookupPath(cue.ParsePath("limit")).Int64(); err == nil {
				rl.WindowLimit = int(v)
			}
			if rl.RPS > 0 || rl.Burst > 0 || rl.WindowLimit > 0 {
				ep.RateLimit = rl
			}
		}

		// Apply default rate limit if endpoint doesn't have explicit one
		if ep.RateLimit == nil && defaultRateLimit != nil {
			ep.RateLimit = defaultRateLimit
		}

		// Parse max_concurrent (backpressure via semaphore)
		if v, err := epVal.LookupPath(cue.ParsePath("max_concurrent")).Int64(); err == nil && v > 0 {
			ep.MaxConcurrent = int(v)
		}

		// Parse coalesce (singleflight deduplication for GET requests)
		if v, _ := epVal.LookupPath(cue.ParsePath("coalesce")).Bool(); v {
			ep.Coalesce = true
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

		retryVal := epVal.LookupPath(cue.ParsePath("retry"))
		if retryVal.Exists() {
			rp := &RetryPolicyDef{
				Enabled:            true,
				MaxAttempts:        3,
				BaseDelayMS:        200,
				RetryOnStatuses:    []int{429, 502, 503, 504},
				RetryNetworkErrors: true,
			}
			if v, err := retryVal.LookupPath(cue.ParsePath("enabled")).Bool(); err == nil {
				rp.Enabled = v
			}
			if v, err := retryVal.LookupPath(cue.ParsePath("max_attempts")).Int64(); err == nil {
				rp.MaxAttempts = int(v)
			}
			if v, err := retryVal.LookupPath(cue.ParsePath("base_delay_ms")).Int64(); err == nil {
				rp.BaseDelayMS = int(v)
			}
			if v, err := retryVal.LookupPath(cue.ParsePath("retry_network_errors")).Bool(); err == nil {
				rp.RetryNetworkErrors = v
			}
			if statuses := retryVal.LookupPath(cue.ParsePath("retry_on_statuses")); statuses.Exists() && statuses.Kind() == cue.ListKind {
				var parsed []int
				it, _ := statuses.List()
				for it.Next() {
					if iv, err := it.Value().Int64(); err == nil {
						parsed = append(parsed, int(iv))
					}
				}
				if len(parsed) > 0 {
					rp.RetryOnStatuses = parsed
				}
			}
			ep.RetryPolicy = rp
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
