package policy

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestFromEndpoint(t *testing.T) {
	t.Parallel()

	ep := normalizer.Endpoint{
		AuthType:    "jwt",
		Permission:  "company.update",
		AuthRoles:   []string{"owner", "admin"},
		CacheTTL:    "24h",
		Timeout:     "30s",
		Idempotency: true,
		MaxBodySize: 1024,
		RateLimit: &normalizer.RateLimitDef{
			RPS:   10,
			Burst: 20,
		},
		CircuitBreaker: &normalizer.CircuitBreakerDef{
			Threshold:   5,
			Timeout:     "45s",
			HalfOpenMax: 2,
		},
	}

	p := FromEndpoint(ep)
	if p.AuthType != "jwt" || p.Permission != "company.update" {
		t.Fatalf("unexpected auth policy: %+v", p)
	}
	if !p.Idempotency || p.Timeout != "30s" || p.CacheTTL != "24h" {
		t.Fatalf("unexpected transport policy: %+v", p)
	}
	if len(p.AuthRoles) != 2 || p.AuthRoles[0] != "owner" || p.AuthRoles[1] != "admin" {
		t.Fatalf("unexpected roles: %+v", p.AuthRoles)
	}
	if p.RateLimit == nil || p.RateLimit.RPS != 10 || p.RateLimit.Burst != 20 {
		t.Fatalf("unexpected rate limit: %+v", p.RateLimit)
	}
	if p.CircuitBreaker == nil || p.CircuitBreaker.Timeout != "45s" {
		t.Fatalf("unexpected circuit breaker: %+v", p.CircuitBreaker)
	}
	if len(p.Validation.RequiredHeaders) != 2 || p.Validation.RequiredHeaders[0] != "Authorization" || p.Validation.RequiredHeaders[1] != "Idempotency-Key" {
		t.Fatalf("unexpected required headers: %+v", p.Validation.RequiredHeaders)
	}
}
