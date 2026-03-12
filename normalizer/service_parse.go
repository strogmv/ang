package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

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
		method.PrimaryOperationKind = parseOperationKind(methodVal)
		method.Capabilities = parseCapabilities(methodVal)
		method.SideEffects = parseSideEffects(methodVal)
		method.ManualRequired = parseManualRequired(methodVal)
		method.Planner = parsePlannerHints(methodVal)
		if streamVal := methodVal.LookupPath(cue.MakePath(cue.Str("stream"))); streamVal.Exists() {
			if b, err := streamVal.Bool(); err == nil {
				method.IsStreaming = b
			}
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
				Name:                 strings.Trim(handler, ""),
				Input:                Entity{Name: evtName},
				PrimaryOperationKind: "",
			}
			svc.Methods = append(svc.Methods, method)
		}
	}
	return svc, nil
}
