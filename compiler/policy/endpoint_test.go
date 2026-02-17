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
	if !p.Retry.Enabled || p.Retry.MaxAttempts != 3 {
		t.Fatalf("unexpected retry policy: %+v", p.Retry)
	}
}

func TestFromEndpoint_UsesRetryOverride(t *testing.T) {
	t.Parallel()

	ep := normalizer.Endpoint{
		Method: "POST",
		RetryPolicy: &normalizer.RetryPolicyDef{
			Enabled:            true,
			MaxAttempts:        5,
			BaseDelayMS:        50,
			RetryOnStatuses:    []int{409, 429},
			RetryNetworkErrors: false,
		},
	}
	p := FromEndpoint(ep)
	if !p.Retry.Enabled || p.Retry.MaxAttempts != 5 || p.Retry.BaseDelayMS != 50 {
		t.Fatalf("unexpected retry override: %+v", p.Retry)
	}
	if len(p.Retry.RetryOnStatuses) != 2 || p.Retry.RetryOnStatuses[0] != 409 {
		t.Fatalf("unexpected retry statuses: %+v", p.Retry.RetryOnStatuses)
	}
	if p.Retry.RetryNetworkErrors {
		t.Fatalf("expected retry network errors false, got true")
	}
}
