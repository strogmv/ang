package emitter

import (
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func resolveRequestRule(t frontendRequestPolicyTable, method, path string) (frontendRequestRule, bool) {
	for _, o := range t.Overrides {
		if o.Method == method && o.Path == path {
			return o.frontendRequestRule, true
		}
	}
	for _, d := range t.Defaults {
		if d.Method == method {
			return d.frontendRequestRule, true
		}
	}
	return frontendRequestRule{Strategy: -1}, false
}

func TestBuildFrontendRequestPolicyTable_EveryRouteResolvesToItsOwnRule(t *testing.T) {
	eps := []normalizer.Endpoint{
		{Method: "GET", Path: "/api/a", RPC: "GetA"},
		{Method: "GET", Path: "/api/b/{id}", RPC: "GetB"},
		{Method: "GET", Path: "/api/live", RPC: "GetLive", Metadata: map[string]any{"cachePolicy": "realtime"}},
		{Method: "POST", Path: "/api/a", RPC: "CreateA"},
		{Method: "POST", Path: "/api/pay", RPC: "Pay", Idempotency: true},
		{Method: "WS", Path: "/ws", RPC: "Live"},
	}
	table := buildFrontendRequestPolicyTable(eps)
	for _, d := range table.Defaults {
		if d.Method == "WS" {
			t.Fatalf("WS endpoints are not HTTP requests: %+v", table)
		}
	}
	live, _ := resolveRequestRule(table, "GET", "/api/live")
	if !live.NoStore {
		t.Fatalf("a realtime GET must bypass caches: %+v", table)
	}
	plain, _ := resolveRequestRule(table, "GET", "/api/a")
	if plain.NoStore {
		t.Fatalf("a plain GET must not bypass caches: %+v", table)
	}
	pay, _ := resolveRequestRule(table, "POST", "/api/pay")
	if !pay.Idempotent {
		t.Fatalf("an idempotent POST must carry an Idempotency-Key: %+v", table)
	}
	create, _ := resolveRequestRule(table, "POST", "/api/a")
	if create.Idempotent || create.Strategy >= 0 {
		t.Fatalf("a plain POST is neither retried nor keyed — not even on the path of a retried GET: %+v", table)
	}
}
