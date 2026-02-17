package policy

import (
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

// EndpointPolicy is a single policy source used by backend and SDK emitters.
type EndpointPolicy struct {
	AuthType       string
	Permission     string
	AuthRoles      []string
	CacheTTL       string
	Timeout        string
	Idempotency    bool
	MaxBodySize    int64
	RateLimit      *normalizer.RateLimitDef
	CircuitBreaker *normalizer.CircuitBreakerDef
	Validation     ValidationPolicy
}

// ValidationPolicy carries endpoint-level validation contract.
type ValidationPolicy struct {
	RequiredHeaders []string
}

// FromEndpoint extracts a normalized policy layer from endpoint contract.
func FromEndpoint(ep normalizer.Endpoint) EndpointPolicy {
	p := EndpointPolicy{
		AuthType:    ep.AuthType,
		Permission:  ep.Permission,
		CacheTTL:    ep.CacheTTL,
		Timeout:     ep.Timeout,
		Idempotency: ep.Idempotency,
		MaxBodySize: ep.MaxBodySize,
	}
	if len(ep.AuthRoles) > 0 {
		p.AuthRoles = append([]string{}, ep.AuthRoles...)
	}
	if ep.RateLimit != nil {
		rl := *ep.RateLimit
		p.RateLimit = &rl
	}
	if ep.CircuitBreaker != nil {
		cb := *ep.CircuitBreaker
		p.CircuitBreaker = &cb
	}
	p.Validation.RequiredHeaders = append(p.Validation.RequiredHeaders, requiredHeaders(ep)...)
	return p
}

func requiredHeaders(ep normalizer.Endpoint) []string {
	var out []string
	if ep.AuthType != "" && ep.AuthType != "none" {
		out = append(out, "Authorization")
	}
	method := strings.ToUpper(ep.Method)
	if ep.Idempotency && method != "GET" && method != "WS" {
		out = append(out, "Idempotency-Key")
	}
	return out
}
