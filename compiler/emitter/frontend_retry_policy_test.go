package emitter

import (
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func TestBuildFrontendRetryPolicyTable_DefaultPerMethodAndOverrides(t *testing.T) {
	eps := []normalizer.Endpoint{
		{Method: "GET", Path: "/api/a", RPC: "GetA"},
		{Method: "GET", Path: "/api/b/{id}", RPC: "GetB"},
		{Method: "POST", Path: "/api/a", RPC: "CreateA"},
		{Method: "WS", Path: "/ws", RPC: "Live"},
	}
	table := buildFrontendRetryPolicyTable(eps)
	byMethod := map[string]int{}
	for _, d := range table.Defaults {
		byMethod[d.Method] = d.Strategy
	}
	if _, ok := byMethod["POST"]; ok {
		t.Fatalf("POST must not get a retry default: %+v", table)
	}
	if _, ok := byMethod["WS"]; ok {
		t.Fatalf("WS endpoints are not HTTP requests: %+v", table)
	}
	for _, o := range table.Overrides {
		if o.Method == "POST" && o.Strategy >= 0 {
			t.Fatalf("a POST must never be retried by an override: %+v", o)
		}
	}
	// Every route resolves to exactly what its policy says.
	for _, ep := range eps[:3] {
		want := -1
		for _, d := range table.Defaults {
			if d.Method == ep.Method {
				want = d.Strategy
			}
		}
		for _, o := range table.Overrides {
			if o.Method == ep.Method && o.Path == ep.Path {
				want = o.Strategy
			}
		}
		got := want >= 0
		if ep.Method == "POST" && got {
			t.Fatalf("%s %s resolved to a retry strategy", ep.Method, ep.Path)
		}
	}
}
