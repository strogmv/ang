package emitter

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

func TestBuildMiddlewareList_RateLimitKeys(t *testing.T) {
	t.Parallel()

	limits := []normalizer.RateLimitDef{
		{Key: "user", RPS: 10, Burst: 20},
		{Key: "ip", RPS: 50, Burst: 100, Window: "1h", WindowLimit: 5000},
		{Key: "company", RPS: 30, Burst: 60},
	}
	ep := normalizer.Endpoint{AuthType: "jwt", RateLimit: &limits[0], RateLimits: limits}
	got := buildMiddlewareList(ep, false, false)
	want := `AuthMiddleware, RateLimitByMiddleware("user", 10, 20, 0, 0), RateLimitMiddleware(50, 100, 3600, 5000), RateLimitByMiddleware("company", 30, 60, 0, 0)`
	if !strings.Contains(got, want) {
		t.Fatalf("middleware chain\n got: %s\nwant: …%s…", got, want)
	}
}

func TestBuildMiddlewareList_AddressLimitUnchanged(t *testing.T) {
	t.Parallel()

	// No key (or "ip") must keep rendering the historical call byte for byte,
	// so projects that never set a key see no diff.
	for _, key := range []string{"", "ip"} {
		rl := normalizer.RateLimitDef{Key: key, RPS: 5, Burst: 20, Window: "1h", WindowLimit: 100}
		got := buildMiddlewareList(normalizer.Endpoint{RateLimit: &rl}, false, false)
		if !strings.Contains(got, "RateLimitMiddleware(5, 20, 3600, 100)") || strings.Contains(got, "RateLimitBy") {
			t.Fatalf("key %q: unexpected chain %s", key, got)
		}
	}
}

func TestRateLimitsSurviveIRRoundTrip(t *testing.T) {
	t.Parallel()

	limits := []normalizer.RateLimitDef{
		{Key: "user", RPS: 10, Burst: 20},
		{RPS: 1, Burst: 5, Window: "15m", WindowLimit: 20},
	}
	irEp := ir.ConvertEndpoint(normalizer.Endpoint{
		Method: "POST", Path: "/x", ServiceName: "s", RPC: "X", AuthType: "jwt",
		RateLimit: &limits[0], RateLimits: limits,
	})
	primary, stack := rateLimitsFromIR(irEp.RateLimit, irEp.RateLimits)
	if primary == nil || *primary != limits[0] {
		t.Fatalf("primary lost in IR round trip: %+v", primary)
	}
	if len(stack) != 2 || stack[1] != limits[1] {
		t.Fatalf("stack lost in IR round trip (window/limit/key must survive): %+v", stack)
	}
	// IR written before the stack existed carries only the primary.
	primary, stack = rateLimitsFromIR(&ir.RateLimit{RPS: 2, Burst: 4, Window: "1h", WindowLimit: 9}, nil)
	if primary.WindowLimit != 9 || len(stack) != 1 {
		t.Fatalf("primary-only IR: %+v %+v", primary, stack)
	}
}

func TestEmitHTTPCommon_RateLimitByIdentity(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	if err := em.EmitHTTPCommon(&normalizer.AuthDef{Mode: "opaque_session_cookie"}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(tmp, "internal", "transport", "http", "common.go")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), file, data, 0); err != nil {
		t.Fatalf("generated common.go does not parse: %v", err)
	}
	generated := string(data)
	for _, want := range []string{
		"func RateLimitMiddleware(rps, burst, windowSecs, windowLimit int) func(http.Handler) http.Handler {\n\treturn rateLimitMiddleware(rps, burst, windowSecs, windowLimit, realClientIP)",
		"func RateLimitByMiddleware(key string, rps, burst, windowSecs, windowLimit int)",
		`return "u:" + id`,
		`return "c:" + id`,
		// Identity buckets and address buckets share one key space per route.
		`clientKey := fmt.Sprintf("%d:%s", scope, bucketOf(r))`,
		// Same store as before: Redis when wired, memory otherwise.
		`key := fmt.Sprintf("rate:rps:%d:%s", max, clientKey)`,
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated common.go is missing %q", want)
		}
	}
}
