package policy

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func TestValidateEndpoint_RateLimitIdentityKeyNeedsAuth(t *testing.T) {
	t.Parallel()

	limits := []normalizer.RateLimitDef{{Key: "user", RPS: 10, Burst: 20}}
	ep := normalizer.Endpoint{Method: "POST", RateLimit: &limits[0], RateLimits: limits}
	if err := ValidateEndpoint(ep); err == nil || !strings.Contains(err.Error(), "needs endpoint auth") {
		t.Fatalf("want auth error for a user-keyed limit on a public route, got %v", err)
	}
	ep.AuthType = "jwt"
	if err := ValidateEndpoint(ep); err != nil {
		t.Fatalf("user-keyed limit with auth must pass: %v", err)
	}
}

func TestValidateEndpoint_RateLimitStackChecksEveryEntry(t *testing.T) {
	t.Parallel()

	limits := []normalizer.RateLimitDef{{Key: "ip", RPS: 10}, {Key: "tenant", RPS: 1}}
	ep := normalizer.Endpoint{Method: "POST", AuthType: "jwt", RateLimit: &limits[0], RateLimits: limits}
	if err := ValidateEndpoint(ep); err == nil || !strings.Contains(err.Error(), `"tenant"`) {
		t.Fatalf("want unknown-key error from the second entry, got %v", err)
	}
	limits = []normalizer.RateLimitDef{{RPS: 10}, {RPS: 1, WindowLimit: -1}}
	ep.RateLimit, ep.RateLimits = &limits[0], limits
	if err := ValidateEndpoint(ep); err == nil {
		t.Fatal("want negative-value error from the second entry")
	}
}

func TestFromEndpoint_CopiesRateLimitStack(t *testing.T) {
	t.Parallel()

	limits := []normalizer.RateLimitDef{{Key: "user", RPS: 10}, {Key: "ip", RPS: 50}}
	p := FromEndpoint(normalizer.Endpoint{RateLimit: &limits[0], RateLimits: limits})
	if len(p.RateLimits) != 2 || p.RateLimit.Key != "user" {
		t.Fatalf("unexpected policy limits %+v / %+v", p.RateLimit, p.RateLimits)
	}
	// Endpoints built by hand (tests, older callers) set only the primary.
	p = FromEndpoint(normalizer.Endpoint{RateLimit: &normalizer.RateLimitDef{RPS: 3}})
	if len(p.RateLimits) != 1 || p.RateLimits[0].RPS != 3 {
		t.Fatalf("primary-only endpoint must yield a one-entry stack, got %+v", p.RateLimits)
	}
}
