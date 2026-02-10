package emitter

import (
	"testing"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/planner"
)

func TestBuildFastAPIPlan_RouterAndServiceStubs(t *testing.T) {
	schema := &ir.Schema{
		Project: ir.Project{Name: "svc", Version: "0.1.0"},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/posts/{id}", Service: "Blog", RPC: "GetPost"},
			{Method: "POST", Path: "/posts", Service: "Blog", RPC: "GetPost"},
			{Method: "WS", Path: "/ws", Service: "Blog", RPC: "Ignored"},
		},
	}

	plan := planner.BuildFastAPIPlan(schema, "0.1.0")
	if len(plan.Routers) != 1 {
		t.Fatalf("expected 1 router, got %d", len(plan.Routers))
	}
	r := plan.Routers[0]
	if r.ModuleName != "blog" {
		t.Fatalf("unexpected module name: %s", r.ModuleName)
	}
	if len(r.Routes) != 2 {
		t.Fatalf("expected 2 HTTP routes, got %d", len(r.Routes))
	}
	if r.Routes[0].HandlerName != "get_post" {
		t.Fatalf("unexpected first handler: %s", r.Routes[0].HandlerName)
	}
	if r.Routes[1].HandlerName != "get_post_post" {
		t.Fatalf("unexpected second handler: %s", r.Routes[1].HandlerName)
	}
	if len(plan.ServiceStubs) != 1 || len(plan.ServiceStubs[0].Methods) != 2 {
		t.Fatalf("unexpected service stubs: %#v", plan.ServiceStubs)
	}
}

func TestBuildFastAPIPlan_PythonImplInjected(t *testing.T) {
	schema := &ir.Schema{
		Project: ir.Project{Name: "svc", Version: "0.1.0"},
		Services: []ir.Service{
			{
				Name: "Report",
				Methods: []ir.Method{
					{
						Name: "GeneratePdf",
						Impl: &ir.Impl{
							Lang:    "python",
							Code:    "return {'ok': True}",
							Imports: []string{"from fastapi import Response"},
						},
					},
				},
			},
		},
		Endpoints: []ir.Endpoint{
			{Method: "POST", Path: "/reports/pdf", Service: "Report", RPC: "GeneratePdf"},
		},
	}

	plan := planner.BuildFastAPIPlan(schema, "0.1.0")
	if len(plan.ServiceStubs) != 1 {
		t.Fatalf("expected 1 service stub, got %d", len(plan.ServiceStubs))
	}
	stub := plan.ServiceStubs[0]
	if len(stub.Imports) != 1 || stub.Imports[0] != "from fastapi import Response" {
		t.Fatalf("unexpected imports: %#v", stub.Imports)
	}
	if len(stub.Methods) != 1 || stub.Methods[0].Body == "" {
		t.Fatalf("expected injected python impl body, got %#v", stub.Methods)
	}
}
