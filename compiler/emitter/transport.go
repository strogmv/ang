package emitter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
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
}

type WsServiceGroup struct {
	Name         string
	Endpoints    []WsEndpointView
	HasBroadcast bool
	HasRooms     bool
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
	if p.RateLimit != nil {
		windowSecs := rateLimitWindowSeconds(p.RateLimit.Window)
		parts = append(parts, fmt.Sprintf("RateLimitMiddleware(%d, %d, %d, %d)",
			p.RateLimit.RPS, p.RateLimit.Burst, windowSecs, p.RateLimit.WindowLimit))
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
	// TimeoutMiddleware must not be applied to WebSocket endpoints: http.TimeoutHandler
	// wraps the ResponseWriter and removes the http.Hijacker interface required for WS upgrade.
	if p.Timeout != "" && !strings.EqualFold(ep.Method, "WS") {
		parts = append(parts, fmt.Sprintf("TimeoutMiddleware(%q)", p.Timeout))
	}
	if includeIdempotency && p.Idempotency {
		parts = append(parts, "IdempotencyMiddleware()")
	}
	return strings.Join(parts, ", ")
}
