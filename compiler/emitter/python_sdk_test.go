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

func TestBuildPythonSDKModels(t *testing.T) {
	entities := []normalizer.Entity{
		{
			Name: "User",
			Fields: []normalizer.Field{
				{Name: "id", Type: "string"},
				{Name: "roles", Type: "string", IsList: true},
				{Name: "meta", Type: "json", IsOptional: true},
			},
		},
	}

	got := buildPythonSDKModels(entities)
	if len(got) != 1 {
		t.Fatalf("expected 1 model, got %d", len(got))
	}
	if got[0].Name != "User" {
		t.Fatalf("unexpected model name: %s", got[0].Name)
	}
	if len(got[0].Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(got[0].Fields))
	}
	if got[0].Fields[0].Type != "str" {
		t.Fatalf("expected id as str, got %s", got[0].Fields[0].Type)
	}
	if got[0].Fields[1].Type != "list[str]" {
		t.Fatalf("expected roles as list[str], got %s", got[0].Fields[1].Type)
	}
	if got[0].Fields[2].Type != "dict[str, Any] | None" {
		t.Fatalf("expected meta optional json, got %s", got[0].Fields[2].Type)
	}
}
