package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
)

func authDenyRouteSchema() *ir.Schema {
	public := func(reason string) map[string]any {
		m := map[string]any{normalizer.EndpointMetaAuthPublic: true}
		if reason != "" {
			m[normalizer.EndpointMetaAuthPublicReason] = reason
		}
		return m
	}
	var methods []ir.Method
	for _, name := range []string{"ListProducts", "TrackOpen", "OpenShare", "Checkout", "Webhook", "Signup", "Me"} {
		methods = append(methods, ir.Method{
			Name:   name,
			Input:  &ir.Entity{Name: name + "Request", Fields: []ir.Field{{Name: "token", Type: ir.TypeRef{Kind: ir.KindString}}}},
			Output: &ir.Entity{Name: name + "Response", Fields: []ir.Field{{Name: "ok", Type: ir.TypeRef{Kind: ir.KindBool}}}},
		})
	}
	return &ir.Schema{
		Services: []ir.Service{{Name: "Shop", Methods: methods}},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/products", Service: "Shop", RPC: "ListProducts"},
			{Method: "GET", Path: "/t/{token}/pixel.gif", Service: "Shop", RPC: "TrackOpen"},
			{Method: "GET", Path: "/share/{token}", Service: "Shop", RPC: "OpenShare"},
			{Method: "POST", Path: "/checkout", Service: "Shop", RPC: "Checkout"},
			{Method: "POST", Path: "/webhook", Service: "Shop", RPC: "Webhook", Metadata: public("signed by the provider")},
			{Method: "POST", Path: "/signup", Service: "Shop", RPC: "Signup", Metadata: public("anyone signs up")},
			{Method: "PUT", Path: "/me", Service: "Shop", RPC: "Me", Auth: &ir.EndpointAuth{Type: "jwt"}},
		},
	}
}

// The routes a deny target generates: every endpoint without auth gets
// AuthMiddleware, the ones public on purpose do not.
func TestAuthDefaultDenyGeneratesAuthMiddleware(t *testing.T) {
	t.Parallel()

	schema := authDenyRouteSchema()
	routes := func(mode string) map[string]string {
		out, res, err := compiler.ApplyAuthDefault(schema, mode)
		if err != nil || res.Err() != nil {
			t.Fatalf("%v %v", err, res.Err())
		}
		root := t.TempDir()
		em := emitter.New(root, filepath.Join(root, "sdk"), "templates")
		if err := em.EmitServiceFromIR(out); err != nil {
			t.Fatal(err)
		}
		if err := em.EmitHTTPFromIR(out, nil); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, "internal", "transport", "http", "shop.go"))
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, line := range strings.Split(string(data), "\n") {
			for _, ep := range schema.Endpoints {
				if strings.Contains(line, "handler"+ep.RPC+"(svc))") && strings.Contains(line, `"`+ep.Path+`"`) {
					got[ep.RPC] = line
				}
			}
		}
		return got
	}
	deny := routes(compiler.AuthDefaultDeny)
	for _, rpc := range []string{"ListProducts", "TrackOpen", "OpenShare", "Checkout", "Me"} {
		if !strings.Contains(deny[rpc], "AuthMiddleware") {
			t.Fatalf("deny: %s must require a caller: %q", rpc, deny[rpc])
		}
	}
	for _, rpc := range []string{"Webhook", "Signup"} {
		if deny[rpc] == "" || strings.Contains(deny[rpc], "AuthMiddleware") {
			t.Fatalf("deny: %s is public on purpose: %q", rpc, deny[rpc])
		}
	}
	allow := routes("")
	for _, rpc := range []string{"ListProducts", "TrackOpen", "OpenShare", "Checkout", "Webhook", "Signup"} {
		if allow[rpc] == "" || strings.Contains(allow[rpc], "AuthMiddleware") {
			t.Fatalf("allow keeps %s public as before: %q", rpc, allow[rpc])
		}
	}
}
