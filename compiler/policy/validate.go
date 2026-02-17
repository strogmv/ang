package policy

import (
	"fmt"
	"strings"
	"time"

	"github.com/strogmv/ang/compiler/normalizer"
)

// ValidateEndpoint checks policy contract conflicts for an endpoint.
func ValidateEndpoint(ep normalizer.Endpoint) error {
	method := strings.ToUpper(strings.TrimSpace(ep.Method))
	if ep.Idempotency && (method == "GET" || method == "WS") {
		return fmt.Errorf("idempotency is not allowed for %s endpoints", method)
	}
	if ep.Timeout != "" {
		if _, err := time.ParseDuration(ep.Timeout); err != nil {
			return fmt.Errorf("invalid timeout %q: %w", ep.Timeout, err)
		}
	}
	if ep.CacheTTL != "" {
		if _, err := time.ParseDuration(ep.CacheTTL); err != nil {
			return fmt.Errorf("invalid cache.ttl %q: %w", ep.CacheTTL, err)
		}
	}
	if ep.MaxBodySize < 0 {
		return fmt.Errorf("max_body_size cannot be negative")
	}
	if ep.RateLimit != nil {
		if ep.RateLimit.RPS < 0 || ep.RateLimit.Burst < 0 {
			return fmt.Errorf("rate_limit values cannot be negative")
		}
	}
	if ep.RetryPolicy != nil {
		if ep.RetryPolicy.MaxAttempts < 0 {
			return fmt.Errorf("retry.max_attempts cannot be negative")
		}
		if ep.RetryPolicy.BaseDelayMS < 0 {
			return fmt.Errorf("retry.base_delay_ms cannot be negative")
		}
		for _, code := range ep.RetryPolicy.RetryOnStatuses {
			if code < 100 || code > 599 {
				return fmt.Errorf("retry.retry_on_statuses contains invalid HTTP code %d", code)
			}
		}
	}
	return nil
}
