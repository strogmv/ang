package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// A 503 the service writes itself (a dependency it knows is unavailable or
// unconfigured) must not open the route's breaker: the caller keeps the real
// error code, and the route is usable again as soon as the dependency is.
func TestEmitHTTPCommon_CircuitBreakerIgnoresServiceUnavailable(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	if err := em.EmitHTTPCommon(&normalizer.AuthDef{Mode: "opaque_session_cookie"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", "common.go"))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(data)
	for _, want := range []string{
		"if breakerCountsFailure(sw.status) {",
		"func breakerCountsFailure(status int) bool {",
		"return status >= 500 && status != http.StatusServiceUnavailable",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated common.go is missing %q", want)
		}
	}
	if strings.Contains(generated, "if sw.status >= 500 {\n\t\t\t\tbreaker.RecordFailure()") {
		t.Fatal("the breaker still counts every 5xx as a failure")
	}
}

// The breaker is built per route: an endpoint without circuit_breaker (a
// read next to a guarded upload) gets no breaker middleware at all, so an
// open upload breaker can never block it.
func TestBuildMiddlewareList_BreakerOnlyOnItsOwnRoute(t *testing.T) {
	t.Parallel()

	upload := normalizer.Endpoint{
		AuthType:       "jwt",
		CircuitBreaker: &normalizer.CircuitBreakerDef{Threshold: 3, Timeout: "60s", HalfOpenMax: 2},
	}
	read := normalizer.Endpoint{AuthType: "jwt"}

	if got := buildMiddlewareList(upload, true, true); !strings.Contains(got, `CircuitBreakerMiddleware(3, "60s", 2)`) {
		t.Fatalf("upload route lost its breaker: %q", got)
	}
	if got := buildMiddlewareList(read, true, true); strings.Contains(got, "CircuitBreakerMiddleware") {
		t.Fatalf("a route without circuit_breaker got a breaker: %q", got)
	}
}
