package policy

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestValidateEndpoint_OK(t *testing.T) {
	t.Parallel()

	ep := normalizer.Endpoint{
		Method:      "POST",
		Timeout:     "30s",
		CacheTTL:    "24h",
		Idempotency: true,
		RateLimit: &normalizer.RateLimitDef{
			RPS:   10,
			Burst: 20,
		},
	}
	if err := ValidateEndpoint(ep); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateEndpoint_IdempotencyConflict(t *testing.T) {
	t.Parallel()

	ep := normalizer.Endpoint{Method: "GET", Idempotency: true}
	err := ValidateEndpoint(ep)
	if err == nil || !strings.Contains(err.Error(), "idempotency") {
		t.Fatalf("expected idempotency conflict, got: %v", err)
	}
}

func TestValidateEndpoint_InvalidTimeout(t *testing.T) {
	t.Parallel()

	ep := normalizer.Endpoint{Method: "POST", Timeout: "30seconds"}
	err := ValidateEndpoint(ep)
	if err == nil || !strings.Contains(err.Error(), "invalid timeout") {
		t.Fatalf("expected timeout validation error, got: %v", err)
	}
}

func TestValidateEndpoint_InvalidRetryStatusCode(t *testing.T) {
	t.Parallel()

	ep := normalizer.Endpoint{
		Method: "POST",
		RetryPolicy: &normalizer.RetryPolicyDef{
			Enabled:         true,
			MaxAttempts:     2,
			BaseDelayMS:     100,
			RetryOnStatuses: []int{42},
		},
	}
	err := ValidateEndpoint(ep)
	if err == nil || !strings.Contains(err.Error(), "retry.retry_on_statuses") {
		t.Fatalf("expected retry status validation error, got: %v", err)
	}
}
