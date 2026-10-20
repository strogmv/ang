package compiler

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// dealingi-back gated its console routes by a prefix rule in a helper
// (adminRouteRight): a route added without thinking about rights fell into
// whatever the prefixes said. Under target.route_rights every route of a
// gated role declares its right, the generator refuses one that does not,
// and a declared right needs a CheckRoleAccess that takes it.
func TestCheckRouteRights(t *testing.T) {
	admin := func(method, path, right string) ir.Endpoint {
		return ir.Endpoint{Method: method, Path: path, RPC: "Op" + right, Source: "cue/api/admin.cue:1",
			Auth: &ir.EndpointAuth{Type: "jwt", Roles: []string{"admin"}, Right: right}}
	}
	gate := ir.Service{Name: "Auth", Methods: []ir.Method{{Name: "CheckRoleAccess", Input: &ir.Entity{Fields: []ir.Field{{Name: "userID"}, {Name: "role"}, {Name: "right"}}}}}}
	def := &normalizer.RouteRightsDef{Roles: []string{"admin"}, Rights: []string{"view", "billing"}}
	ok := &ir.Schema{Services: []ir.Service{gate}, Endpoints: []ir.Endpoint{
		admin("GET", "/api/admin/users", "view"),
		admin("POST", "/api/admin/billing/runs", "billing"),
		{Method: "WS", Path: "/ws/admin", Auth: &ir.EndpointAuth{Type: "jwt", Roles: []string{"admin"}}},
		{Method: "GET", Path: "/api/me", Auth: &ir.EndpointAuth{Type: "jwt", Roles: []string{"owner"}}},
	}}
	if err := CheckRouteRights(ok, def); err != nil {
		t.Fatal(err)
	}
	bad := &ir.Schema{Services: []ir.Service{gate}, Endpoints: append(append([]ir.Endpoint{}, ok.Endpoints...),
		admin("DELETE", "/api/admin/users/{id}", ""), admin("POST", "/api/admin/x", "bilyng"))}
	err := CheckRouteRights(bad, def)
	if err == nil || !strings.Contains(err.Error(), CodeAuthRightMissing) || !strings.Contains(err.Error(), "cue/api/admin.cue:1: Op DELETE /api/admin/users/{id}") ||
		!strings.Contains(err.Error(), CodeAuthRightUnknown) || !strings.Contains(err.Error(), `"bilyng"`) {
		t.Fatalf("missing and unknown rights: %v", err)
	}
	unchecked := &ir.Schema{Endpoints: ok.Endpoints}
	if err := CheckRouteRights(unchecked, nil); err == nil || !strings.Contains(err.Error(), CodeAuthRightUnchecked) {
		t.Fatalf("a right nothing checks: %v", err)
	}
	if err := CheckRouteRights(&ir.Schema{Endpoints: []ir.Endpoint{{Method: "GET", Path: "/a", Auth: &ir.EndpointAuth{Type: "jwt", Roles: []string{"admin"}}}}}, nil); err != nil {
		t.Fatalf("without target.route_rights and rights nothing is required: %v", err)
	}
}
