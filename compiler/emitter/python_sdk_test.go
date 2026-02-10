package emitter

import (
	"testing"

	pyemitter "github.com/strogmv/ang/compiler/emitter/python"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestPathParamNames(t *testing.T) {
	got := pyemitter.PathParamNames("/v1/users/{id}/orders/{order_id}/{id}")
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

	got := pyemitter.BuildEndpoints(eps, nil)
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

func TestBuildPythonEndpointsTypedFromServiceMethods(t *testing.T) {
	schema := &ir.Schema{
		Services: []ir.Service{
			{
				Name: "Auth",
				Methods: []ir.Method{
					{
						Name:   "Login",
						Input:  &ir.Entity{Name: "LoginRequest"},
						Output: &ir.Entity{Name: "AuthTokens"},
					},
				},
			},
		},
		Endpoints: []ir.Endpoint{
			{Method: "POST", Path: "/auth/login", Service: "Auth", RPC: "Login"},
		},
	}

	got := pyemitter.BuildEndpointsFromIR(schema)
	if len(got) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(got))
	}
	if got[0].PayloadType != "models.LoginRequest" {
		t.Fatalf("unexpected payload type: %s", got[0].PayloadType)
	}
	if got[0].ReturnType != "models.AuthTokens" {
		t.Fatalf("unexpected return type: %s", got[0].ReturnType)
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

	got := pyemitter.BuildSDKModels(entities)
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

func TestBuildPythonSDKModels_AliasAndEntityRefs(t *testing.T) {
	entities := []normalizer.Entity{
		{
			Name: "Team",
			Fields: []normalizer.Field{
				{Name: "id", Type: "domain.ID"},
				{Name: "owner", Type: "domain.User"},
				{Name: "members", Type: "[]domain.User"},
				{Name: "meta", Type: "map[string]any"},
			},
		},
		{
			Name: "User",
			Fields: []normalizer.Field{
				{Name: "id", Type: "domain.ID"},
				{Name: "email", Type: "domain.Email"},
			},
		},
	}

	got := pyemitter.BuildSDKModels(entities)
	if len(got) != 2 {
		t.Fatalf("expected 2 models, got %d", len(got))
	}

	var team pyemitter.SDKModel
	for _, m := range got {
		if m.Name == "Team" {
			team = m
			break
		}
	}
	if team.Name == "" {
		t.Fatalf("team model not found")
	}
	if len(team.Fields) != 4 {
		t.Fatalf("expected 4 team fields, got %d", len(team.Fields))
	}
	if team.Fields[0].Type != "str" {
		t.Fatalf("expected id as str, got %s", team.Fields[0].Type)
	}
	if team.Fields[1].Type != "User" {
		t.Fatalf("expected owner as User, got %s", team.Fields[1].Type)
	}
	if team.Fields[2].Type != "list[User]" {
		t.Fatalf("expected members as list[User], got %s", team.Fields[2].Type)
	}
	if team.Fields[3].Type != "dict[str, Any]" {
		t.Fatalf("expected meta as dict[str, Any], got %s", team.Fields[3].Type)
	}
}
