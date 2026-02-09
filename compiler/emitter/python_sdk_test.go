package emitter

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestPathParamNames(t *testing.T) {
	got := pathParamNames("/v1/users/{id}/orders/{order_id}/{id}")
	if len(got) != 2 {
		t.Fatalf("expected 2 unique params, got %d", len(got))
	}
	if got[0] != "id" || got[1] != "order_id" {
		t.Fatalf("unexpected params: %#v", got)
	}
}

func TestBuildPythonEndpointsUniqueNames(t *testing.T) {
	eps := []normalizer.Endpoint{
		{Method: "GET", Path: "/users/{id}", RPC: "FindUser"},
		{Method: "POST", Path: "/users/{id}", RPC: "FindUser"},
		{Method: "WS", Path: "/ws", RPC: "WsIgnored"},
	}

	got := buildPythonEndpoints(eps)
	if len(got) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(got))
	}

	if got[0].MethodName != "find_user" {
		t.Fatalf("unexpected first method name: %s", got[0].MethodName)
	}
	if got[1].MethodName != "find_user_post" {
		t.Fatalf("unexpected second method name: %s", got[1].MethodName)
	}
}
