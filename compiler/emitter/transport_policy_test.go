package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestBuildMiddlewareList_UsesPolicyLayer(t *testing.T) {
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

	got := buildMiddlewareList(ep, true, true)
	for _, expected := range []string{
		"MaxBodySizeMiddleware(1024)",
		"AuthMiddleware",
		`RequireRoles([]string{"owner", "admin"})`,
		`RequirePermission("company.update")`,
		`CacheMiddleware("24h")`,
		"RateLimitMiddleware(10, 20)",
		`CircuitBreakerMiddleware(5, "45s", 2)`,
		`TimeoutMiddleware("30s")`,
		"IdempotencyMiddleware()",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("missing middleware %q in %q", expected, got)
		}
	}
}
