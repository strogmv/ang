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

// frontendRetryMethodDefault is the rule most operations of an HTTP method share.
type frontendRetryMethodDefault struct {
	Method   string
	Strategy int
}

// frontendRetryRoute is an operation whose rule differs from its method's
// default; Strategy -1 means it is never retried.
type frontendRetryRoute struct {
	Method   string
	Path     string
	Strategy int
}

// frontendRetryPolicyTable is what endpoints/retry-policy.ts carries: the API
// client is part of every page, so it gets these few rules instead of the
// metadata of every operation (endpoints/meta.ts).
type frontendRetryPolicyTable struct {
	Strategies []frontendRetryStrategy
	Defaults   []frontendRetryMethodDefault
	Overrides  []frontendRetryRoute
}

func buildFrontendRetryPolicyTable(eps []normalizer.Endpoint) frontendRetryPolicyTable {
	var table frontendRetryPolicyTable
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

	type route struct {
		method, path string
		strategy     int
	}
	var routes []route
	counts := map[string]map[int]int{}
	for _, ep := range eps {
		method := strings.ToUpper(strings.TrimSpace(ep.Method))
		if method == "" || method == "WS" {
			continue
		}
		s := strategyOf(policy.FromEndpoint(ep).Retry)
		routes = append(routes, route{method, ep.Path, s})
		if counts[method] == nil {
			counts[method] = map[int]int{}
		}
		counts[method][s]++
	}

	defaults := map[string]int{}
	methods := make([]string, 0, len(counts))
	for m := range counts {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	for _, m := range methods {
		best, bestCount := -1, -1
		for s, c := range counts[m] {
			if c > bestCount || (c == bestCount && s < best) {
				best, bestCount = s, c
			}
		}
		defaults[m] = best
		if best >= 0 {
			table.Defaults = append(table.Defaults, frontendRetryMethodDefault{Method: m, Strategy: best})
		}
	}
	for _, r := range routes {
		if r.strategy != defaults[r.method] {
			table.Overrides = append(table.Overrides, frontendRetryRoute{Method: r.method, Path: r.path, Strategy: r.strategy})
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
