package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

// A declared right reaches the gate: the route is wrapped in
// RequireRolesRight, the role access checker gets the right, the table
// RouteRight answers it; routes without one keep RequireRoles.
func TestRouteRightReachesTheRoleGate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.GoModule = "example.com/shop"
	ok := ir.TypeRef{Kind: ir.KindBool}
	schema := &ir.Schema{
		Services: []ir.Service{{Name: "Admin", Methods: []ir.Method{
			{Name: "ListInvoices", Input: &ir.Entity{Name: "ListInvoicesRequest"}, Output: &ir.Entity{Name: "ListInvoicesResponse", Fields: []ir.Field{{Name: "ok", Type: ok}}}},
			{Name: "ListUsers", Input: &ir.Entity{Name: "ListUsersRequest"}, Output: &ir.Entity{Name: "ListUsersResponse", Fields: []ir.Field{{Name: "ok", Type: ok}}}},
		}}},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/api/admin/billing/invoices", Service: "Admin", RPC: "ListInvoices", Auth: &ir.EndpointAuth{Type: "jwt", Roles: []string{"admin"}, Right: "billing"}},
			{Method: "GET", Path: "/api/admin/users", Service: "Admin", RPC: "ListUsers", Auth: &ir.EndpointAuth{Type: "jwt", Roles: []string{"admin"}}},
		},
	}
	em.IRSchema = schema
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitHTTPFromIR(schema, nil); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(root, "internal", "transport", "http", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	routes := read("admin.go")
	if !strings.Contains(routes, `RequireRolesRight([]string{"admin"}, "billing")`) || !strings.Contains(routes, `RequireRoles([]string{"admin"})`) {
		t.Fatalf("route gates:\n%s", routes)
	}
	table := read("route_rights.go")
	if !strings.Contains(table, `"GET /api/admin/billing/invoices": "billing",`) || strings.Contains(table, "/api/admin/users") {
		t.Fatalf("route rights table:\n%s", table)
	}
	common := read("common.go")
	if !strings.Contains(common, "roleAccessChecker(r.Context(), CurrentUserID(r), role, right)") {
		t.Fatal("the role gate hands the right to the checker")
	}
}
