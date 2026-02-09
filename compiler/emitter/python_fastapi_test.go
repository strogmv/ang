package emitter

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestBuildPythonFastAPIData_RouterAndServiceStubs(t *testing.T) {
	endpoints := []normalizer.Endpoint{
		{Method: "GET", Path: "/posts/{id}", ServiceName: "Blog", RPC: "GetPost"},
		{Method: "POST", Path: "/posts", ServiceName: "Blog", RPC: "GetPost"},
		{Method: "WS", Path: "/ws", ServiceName: "Blog", RPC: "Ignored"},
	}

	data := buildPythonFastAPIData(nil, endpoints, nil, nil, "0.1.0")
	if len(data.Routers) != 1 {
		t.Fatalf("expected 1 router, got %d", len(data.Routers))
	}
	r := data.Routers[0]
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
	if len(data.ServiceStubs) != 1 || len(data.ServiceStubs[0].Methods) != 2 {
		t.Fatalf("unexpected service stubs: %#v", data.ServiceStubs)
	}
}
