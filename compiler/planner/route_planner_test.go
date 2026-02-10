package planner

import (
	"testing"

	"github.com/strogmv/ang/compiler/ir"
)

func TestBuildFastAPIPlan_DeduplicatesHandlersAndBuildsRoutes(t *testing.T) {
	schema := &ir.Schema{
		Project: ir.Project{Name: "svc", Version: "0.2.0"},
		Services: []ir.Service{{
			Name: "User",
			Methods: []ir.Method{
				{Name: "Create"},
				{Name: "Create"},
			},
		}},
		Endpoints: []ir.Endpoint{
			{Method: "POST", Path: "/users", Service: "User", RPC: "Create"},
			{Method: "GET", Path: "/users/{id}", Service: "User", RPC: "Create"},
		},
	}

	plan := BuildFastAPIPlan(schema, "")
	if len(plan.Routers) != 1 {
		t.Fatalf("expected one router, got %d", len(plan.Routers))
	}
	routes := plan.Routers[0].Routes
	if len(routes) != 2 {
		t.Fatalf("expected two routes, got %d", len(routes))
	}
	if routes[0].HandlerName == routes[1].HandlerName {
		t.Fatalf("expected deduplicated handler names")
	}
}

func TestBuildModelPlans_FromIR(t *testing.T) {
	models := BuildModelPlans([]ir.Entity{{
		Name: "User",
		Fields: []ir.Field{
			{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
			{Name: "age", Type: ir.TypeRef{Kind: ir.KindInt}, Optional: true},
		},
	}})
	if len(models) != 1 {
		t.Fatalf("expected one model")
	}
	if models[0].Name != "User" {
		t.Fatalf("unexpected model name: %s", models[0].Name)
	}
	if len(models[0].Fields) != 2 {
		t.Fatalf("expected two fields")
	}
}
