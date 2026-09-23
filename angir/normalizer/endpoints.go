package normalizer

import (
	"fmt"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

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
		defaultRateLimit = parseRateLimitDef(defaultRLVal)
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
			Metadata:    map[string]any{},
			Source:      formatPos(epVal),
		}
		if rb := strings.TrimSpace(getString(epVal, "request_body")); rb != "" {
			ep.Metadata["request_body"] = rb
		}
		mergeEndpointMetadata(ep.Metadata, opInfo.value)
		mergeEndpointMetadata(ep.Metadata, epVal)
		if len(ep.Metadata) == 0 {
			ep.Metadata = nil
		}
		if streamVal := opInfo.value.LookupPath(cue.MakePath(cue.Str("stream"))); streamVal.Exists() {
			if b, err := streamVal.Bool(); err == nil {
				ep.IsStreaming = b
			}
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
			limits, err := parseRateLimits(rlVal)
			if err != nil {
				return nil, fmt.Errorf("HTTP endpoint %s: %w", epName, err)
			}
			if len(limits) > 0 {
				ep.RateLimits = limits
				ep.RateLimit = &ep.RateLimits[0]
			}
		}

		// Apply default rate limit if endpoint doesn't have explicit one
		if ep.RateLimit == nil && defaultRateLimit != nil {
			rl := *defaultRateLimit
			ep.RateLimits = []RateLimitDef{rl}
			ep.RateLimit = &ep.RateLimits[0]
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

func mergeEndpointMetadata(dst map[string]any, src cue.Value) {
	for _, key := range []string{"meta", "metadata"} {
		v := src.LookupPath(cue.ParsePath(key))
		if !v.Exists() {
			continue
		}
		raw := cueValueToJSONCompatible(v)
		if m, ok := raw.(map[string]any); ok {
			mergeJSONMaps(dst, m)
			continue
		}
		dst[key] = raw
	}
}

func mergeJSONMaps(dst, src map[string]any) {
	for k, v := range src {
		if existing, ok := dst[k].(map[string]any); ok {
			if incoming, ok := v.(map[string]any); ok {
				mergeJSONMaps(existing, incoming)
				continue
			}
		}
		dst[k] = v
	}
}

// parseRateLimitDef reads one rate_limit entry.
func parseRateLimitDef(v cue.Value) *RateLimitDef {
	rl := &RateLimitDef{}
	if x, err := v.LookupPath(cue.ParsePath("rps")).Int64(); err == nil {
		rl.RPS = int(x)
	}
	if x, err := v.LookupPath(cue.ParsePath("burst")).Int64(); err == nil {
		rl.Burst = int(x)
	}
	if x, err := v.LookupPath(cue.ParsePath("window")).String(); err == nil {
		rl.Window = x
	}
	if x, err := v.LookupPath(cue.ParsePath("limit")).Int64(); err == nil {
		rl.WindowLimit = int(x)
	}
	if x, err := v.LookupPath(cue.ParsePath("key")).String(); err == nil {
		rl.Key = strings.TrimSpace(x)
	}
	return rl
}

// parseRateLimits reads rate_limit as one entry or a list of entries. Entries
// that limit nothing are dropped; the order is kept, so the first entry is the
// endpoint's primary limit (the one OpenAPI and the SDK describe).
func parseRateLimits(v cue.Value) ([]RateLimitDef, error) {
	var raw []*RateLimitDef
	if v.IncompleteKind() == cue.ListKind {
		it, err := v.List()
		if err != nil {
			return nil, fmt.Errorf("rate_limit: %w", err)
		}
		for it.Next() {
			raw = append(raw, parseRateLimitDef(it.Value()))
		}
	} else {
		raw = append(raw, parseRateLimitDef(v))
	}
	var out []RateLimitDef
	for _, rl := range raw {
		switch rl.Key {
		case "", RateLimitKeyIP, RateLimitKeyUser, RateLimitKeyCompany:
		default:
			return nil, fmt.Errorf("rate_limit key %q: want \"ip\", \"user\" or \"company\"", rl.Key)
		}
		if rl.RPS > 0 || rl.Burst > 0 || rl.WindowLimit > 0 {
			out = append(out, *rl)
		}
	}
	return out, nil
}
