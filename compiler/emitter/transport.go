package emitter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/policy"
)

type HttpEndpointView struct {
	normalizer.Endpoint
	Input                 normalizer.Entity
	Output                normalizer.Entity
	Broadcasts            []normalizer.Entity
	RoomField             string
	AuthCheckHasCompanyID bool
	HasBodyField          bool
}

type HttpServiceGroup struct {
	Name           string
	Endpoints      []HttpEndpointView
	HasViews       bool
	HasQueryParse  bool
	HasETag        bool
	HasStreaming   bool
	HasBroadcast   bool
	HasDomainUsage bool
}

type WsEndpointView struct {
	normalizer.Endpoint
	Broadcasts            []normalizer.Entity
	Input                 normalizer.Entity
	RoomParam             string
	RoomField             string
	AllowDynamicRooms     bool
	AuthCheckHasCompanyID bool
	// AuthCheckInput is the request of the auth.check operation. Its fields
	// named after path params are filled from the path; AuthCheckCompanyField
	// and AuthCheckUserField (Go names, "" when absent) take the caller's
	// company and user from the handshake. The check runs before the upgrade.
	AuthCheckInput        normalizer.Entity
	AuthCheckCompanyField string
	AuthCheckUserField    string
}

type WsServiceGroup struct {
	Name         string
	Endpoints    []WsEndpointView
	HasBroadcast bool
	HasRooms     bool
	HasAuthCheck bool
}

func buildRequireRoles(roles []string) string {
	quoted := make([]string, 0, len(roles))
	for _, role := range roles {
		quoted = append(quoted, fmt.Sprintf("%q", role))
	}
	return fmt.Sprintf("RequireRoles([]string{%s})", strings.Join(quoted, ", "))
}

func buildRequireScopes(scopes []string) string {
	quoted := make([]string, 0, len(scopes))
	for _, s := range scopes {
		quoted = append(quoted, fmt.Sprintf("%q", s))
	}
	return fmt.Sprintf("RequireScopeMiddleware([]string{%s})", strings.Join(quoted, ", "))
}

// formatIntSlice formats a []int as a Go int-slice literal, e.g. []int{429, 502, 503, 504}.
func formatIntSlice(ints []int) string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return "[]int{" + strings.Join(strs, ", ") + "}"
}

// buildRateLimitMiddleware renders one rate limit. An address-keyed limit keeps
// the historical RateLimitMiddleware call; user/company limits go through
// RateLimitByMiddleware, which reads the identity AuthMiddleware stored (it
// runs earlier in the chain) and falls back to the address without one.
func buildRateLimitMiddleware(rl normalizer.RateLimitDef) string {
	windowSecs := rateLimitWindowSeconds(rl.Window)
	if key := rl.KeyOrIP(); key != normalizer.RateLimitKeyIP {
		return fmt.Sprintf("RateLimitByMiddleware(%q, %d, %d, %d, %d)",
			key, rl.RPS, rl.Burst, windowSecs, rl.WindowLimit)
	}
	return fmt.Sprintf("RateLimitMiddleware(%d, %d, %d, %d)",
		rl.RPS, rl.Burst, windowSecs, rl.WindowLimit)
}

func buildMiddlewareList(ep normalizer.Endpoint, includeCache, includeIdempotency bool) string {
	return buildMiddlewareListFull(ep, includeCache, includeIdempotency, false)
}

// buildMiddlewareListFull builds the middleware chain. When skipAuth is true, AuthMiddleware
// is omitted — used for WebSocket routes where auth happens post-upgrade via the auth frame.
func buildMiddlewareListFull(ep normalizer.Endpoint, includeCache, includeIdempotency, skipAuth bool) string {
	p := policy.FromEndpoint(ep)
	var parts []string
	if p.MaxBodySize > 0 {
		parts = append(parts, fmt.Sprintf("MaxBodySizeMiddleware(%d)", p.MaxBodySize))
	}
	if p.AuthType != "" && !skipAuth {
		parts = append(parts, "AuthMiddleware")
		if len(p.AuthRoles) > 0 {
			parts = append(parts, buildRequireRoles(p.AuthRoles))
		}
		if p.Permission != "" {
			parts = append(parts, fmt.Sprintf("RequirePermission(%q)", p.Permission))
		}
	}
	if len(ep.RequiredScopes) > 0 {
		parts = append(parts, buildRequireScopes(ep.RequiredScopes))
	}
	if includeCache && p.CacheTTL != "" {
		parts = append(parts, fmt.Sprintf("CacheMiddleware(%q)", p.CacheTTL))
	}
	for _, rl := range p.RateLimits {
		parts = append(parts, buildRateLimitMiddleware(rl))
	}
	if ep.Coalesce {
		parts = append(parts, "SingleflightMiddleware()")
	}
	if ep.MaxConcurrent > 0 {
		parts = append(parts, fmt.Sprintf("ConcurrencyMiddleware(%d)", ep.MaxConcurrent))
	}
	if p.CircuitBreaker != nil {
		parts = append(parts, fmt.Sprintf("CircuitBreakerMiddleware(%d, %q, %d)", p.CircuitBreaker.Threshold, p.CircuitBreaker.Timeout, p.CircuitBreaker.HalfOpenMax))
	}
	if ep.RetryPolicy != nil && ep.RetryPolicy.Enabled {
		parts = append(parts, fmt.Sprintf("RetryMiddleware(%d, %d, %s)",
			ep.RetryPolicy.MaxAttempts, ep.RetryPolicy.BaseDelayMS,
			formatIntSlice(ep.RetryPolicy.RetryOnStatuses)))
	}
	// TimeoutMiddleware must not be applied to streaming endpoints: http.TimeoutHandler
	// wraps the ResponseWriter and strips interfaces like http.Flusher/http.Hijacker
	// required by SSE and WebSocket handlers.
	if p.Timeout != "" && !strings.EqualFold(ep.Method, "WS") && !ep.IsStreaming {
		parts = append(parts, fmt.Sprintf("TimeoutMiddleware(%q)", p.Timeout))
	}
	if includeIdempotency && p.Idempotency {
		parts = append(parts, "IdempotencyMiddleware()")
	}
	return strings.Join(parts, ", ")
}
