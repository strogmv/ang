package normalizer

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func extractRateLimitEndpoints(t *testing.T, httpBody string) ([]Endpoint, error) {
	t.Helper()
	v := cuecontext.New().CompileString(`package api

PlaceBid: { service: "bids" }
Login: { service: "auth" }

HTTP: {
` + httpBody + `
}`)
	if err := v.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}
	return New().ExtractEndpoints(v)
}

func endpointByRPC(t *testing.T, eps []Endpoint, rpc string) Endpoint {
	t.Helper()
	for _, ep := range eps {
		if ep.RPC == rpc {
			return ep
		}
	}
	t.Fatalf("endpoint %s not found", rpc)
	return Endpoint{}
}

func TestExtractEndpoints_RateLimitKeyAndStack(t *testing.T) {
	t.Parallel()

	eps, err := extractRateLimitEndpoints(t, `
  default_rate_limit: {rps: 100, burst: 200}
  PlaceBid: {
    method: "POST"
    path: "/api/bids"
    auth: {type: "jwt"}
    rate_limit: [
      {key: "user", rps: 10, burst: 20},
      {key: "ip", rps: 50, burst: 100, window: "1h", limit: 5000},
    ]
  }
  Login: {
    method: "POST"
    path: "/api/login"
    rate_limit: {rps: 5, burst: 20, window: "1h", limit: 100}
  }`)
	if err != nil {
		t.Fatal(err)
	}

	bid := endpointByRPC(t, eps, "PlaceBid")
	if len(bid.RateLimits) != 2 {
		t.Fatalf("want 2 stacked limits, got %+v", bid.RateLimits)
	}
	if bid.RateLimit == nil || *bid.RateLimit != bid.RateLimits[0] {
		t.Fatalf("primary limit must be the first entry, got %+v", bid.RateLimit)
	}
	if got := bid.RateLimits[0]; got.Key != "user" || got.RPS != 10 || got.Burst != 20 {
		t.Fatalf("unexpected user limit %+v", got)
	}
	if got := bid.RateLimits[1]; got.Key != "ip" || got.Window != "1h" || got.WindowLimit != 5000 {
		t.Fatalf("unexpected ip limit %+v", got)
	}

	login := endpointByRPC(t, eps, "Login")
	if len(login.RateLimits) != 1 || login.RateLimit.KeyOrIP() != RateLimitKeyIP {
		t.Fatalf("a single limit without key stays an address limit, got %+v", login.RateLimits)
	}
	if login.RateLimit.Window != "1h" || login.RateLimit.WindowLimit != 100 {
		t.Fatalf("window quota lost: %+v", login.RateLimit)
	}
}

func TestExtractEndpoints_RateLimitDefaultFillsStack(t *testing.T) {
	t.Parallel()

	eps, err := extractRateLimitEndpoints(t, `
  default_rate_limit: {rps: 100, burst: 200}
  Login: {method: "POST", path: "/api/login"}`)
	if err != nil {
		t.Fatal(err)
	}
	login := endpointByRPC(t, eps, "Login")
	if login.RateLimit == nil || len(login.RateLimits) != 1 || login.RateLimits[0].RPS != 100 {
		t.Fatalf("default limit must fill primary and stack, got %+v / %+v", login.RateLimit, login.RateLimits)
	}
}

func TestExtractEndpoints_RateLimitRejectsUnknownKey(t *testing.T) {
	t.Parallel()

	_, err := extractRateLimitEndpoints(t, `
  Login: {method: "POST", path: "/api/login", rate_limit: {key: "session", rps: 1}}`)
	if err == nil || !strings.Contains(err.Error(), `rate_limit key "session"`) {
		t.Fatalf("want unknown-key error, got %v", err)
	}
}
