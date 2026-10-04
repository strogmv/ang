package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/policy"
)

// frontendRetryStrategy is one distinct retry rule of the generated API client.
type frontendRetryStrategy struct {
	MaxAttempts        int
	BaseDelayMS        int
	RetryOnStatuses    string // "429, 502, 503, 504"
	RetryNetworkErrors bool
}

// frontendRequestRule is what the API client decides per request: the retry
// strategy (index into Strategies, -1 = never), whether a GET must bypass every
// cache (cachePolicy realtime/bypass/no-store) and whether a write carries an
// Idempotency-Key.
type frontendRequestRule struct {
	Strategy   int
	NoStore    bool
	Idempotent bool
}

type frontendRequestMethodDefault struct {
	Method string
	frontendRequestRule
}

type frontendRequestRoute struct {
	Method string
	Path   string
	frontendRequestRule
}

// frontendRequestPolicyTable is what endpoints/request-policy.ts carries: the
// API client is part of every page, so it gets these few rules — a default per
// HTTP method and the operations that differ — instead of the metadata of every
// operation (endpoints/meta.ts).
type frontendRequestPolicyTable struct {
	Strategies []frontendRetryStrategy
	Defaults   []frontendRequestMethodDefault
	Overrides  []frontendRequestRoute
}

func buildFrontendRequestPolicyTable(eps []normalizer.Endpoint) frontendRequestPolicyTable {
	var table frontendRequestPolicyTable
	index := map[string]int{}
	strategyOf := func(r policy.RetryPolicy) int {
		if !r.Enabled {
			return -1
		}
		statuses := make([]string, len(r.RetryOnStatuses))
		for i, s := range r.RetryOnStatuses {
			statuses[i] = fmt.Sprint(s)
		}
		s := frontendRetryStrategy{
			MaxAttempts:        r.MaxAttempts,
			BaseDelayMS:        r.BaseDelayMS,
			RetryOnStatuses:    strings.Join(statuses, ", "),
			RetryNetworkErrors: r.RetryNetworkErrors,
		}
		key := fmt.Sprintf("%d|%d|%s|%t", s.MaxAttempts, s.BaseDelayMS, s.RetryOnStatuses, s.RetryNetworkErrors)
		if i, ok := index[key]; ok {
			return i
		}
		index[key] = len(table.Strategies)
		table.Strategies = append(table.Strategies, s)
		return index[key]
	}

	var routes []frontendRequestRoute
	counts := map[string]map[frontendRequestRule]int{}
	for _, ep := range eps {
		method := strings.ToUpper(strings.TrimSpace(ep.Method))
		if method == "" || method == "WS" {
			continue
		}
		p := policy.FromEndpoint(ep)
		cache := endpointCachePolicy(ep)
		rule := frontendRequestRule{
			Strategy:   strategyOf(p.Retry),
			NoStore:    cache == "realtime" || cache == "bypass" || cache == "no-store",
			Idempotent: p.Idempotency,
		}
		routes = append(routes, frontendRequestRoute{Method: method, Path: ep.Path, frontendRequestRule: rule})
		if counts[method] == nil {
			counts[method] = map[frontendRequestRule]int{}
		}
		counts[method][rule]++
	}

	less := func(a, b frontendRequestRule) bool {
		if a.Strategy != b.Strategy {
			return a.Strategy < b.Strategy
		}
		if a.NoStore != b.NoStore {
			return !a.NoStore
		}
		return !a.Idempotent && b.Idempotent
	}
	methods := make([]string, 0, len(counts))
	for m := range counts {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	defaults := map[string]frontendRequestRule{}
	for _, m := range methods {
		var best frontendRequestRule
		bestCount := -1
		for rule, c := range counts[m] {
			if c > bestCount || (c == bestCount && less(rule, best)) {
				best, bestCount = rule, c
			}
		}
		defaults[m] = best
		table.Defaults = append(table.Defaults, frontendRequestMethodDefault{Method: m, frontendRequestRule: best})
	}
	for _, r := range routes {
		if r.frontendRequestRule != defaults[r.Method] {
			table.Overrides = append(table.Overrides, r)
		}
	}
	sort.Slice(table.Overrides, func(i, j int) bool {
		if table.Overrides[i].Method != table.Overrides[j].Method {
			return table.Overrides[i].Method < table.Overrides[j].Method
		}
		return table.Overrides[i].Path < table.Overrides[j].Path
	})
	return table
}
