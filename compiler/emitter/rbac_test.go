package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// internal/pkg/rbac used to be generated for every project: an empty role
// table that nothing read (dealingi-back), while RequirePermission still
// consulted it. It exists now only when something uses it.
func TestRBACPackageOnlyWhenUsed(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		rbac   *normalizer.RBACDef
		schema *ir.Schema
		want   bool
	}{
		"nothing":             {schema: &ir.Schema{Endpoints: []ir.Endpoint{{Method: "GET", Path: "/a", Auth: &ir.EndpointAuth{Type: "jwt", Roles: []string{"admin"}}}}}},
		"empty table":         {rbac: &normalizer.RBACDef{Roles: map[string][]string{}}, schema: &ir.Schema{}},
		"declared roles":      {rbac: &normalizer.RBACDef{Roles: map[string][]string{"editor": {"post.write"}}}, schema: &ir.Schema{}, want: true},
		"endpoint permission": {schema: &ir.Schema{Endpoints: []ir.Endpoint{{Method: "POST", Path: "/a", Auth: &ir.EndpointAuth{Type: "jwt", Permission: "post.write"}}}}, want: true},
		"flow step": {schema: &ir.Schema{Services: []ir.Service{{Name: "Blog", Methods: []ir.Method{{Name: "Write", Flow: []ir.FlowStep{
			{Action: "flow.If", Args: map[string]any{"then": []any{map[string]any{"action": "rbac.CheckPermission"}}}},
		}}}}}}, want: true},
		"raw go": {schema: &ir.Schema{Services: []ir.Service{{Name: "Blog", Methods: []ir.Method{{Name: "Write", Impl: &ir.Impl{Code: "if !rbac.CheckPermission(u.Role, \"x\") {}"}}}}}}, want: true},
	} {
		if got := RBACUsed(c.rbac, c.schema); got != c.want {
			t.Errorf("%s: RBACUsed = %v, want %v", name, got, c.want)
		}
	}

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	em.IRSchema = &ir.Schema{}
	if err := em.EmitRBAC(&normalizer.RBACDef{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "internal", "pkg", "rbac")); !os.IsNotExist(err) {
		t.Fatalf("an unused rbac package was generated: %v", err)
	}
	if err := em.EmitHTTPCommon(nil); err != nil {
		t.Fatal(err)
	}
	common, err := os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", "common.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(common), "internal/pkg/rbac") || strings.Contains(string(common), "rbac.CheckPermission") {
		t.Fatal("common.go still imports the package that is not generated")
	}
	if !strings.Contains(string(common), "func RequirePermission(") {
		t.Fatal("RequirePermission (token permissions) must stay")
	}

	em.RBACDef = &normalizer.RBACDef{Roles: map[string][]string{"editor": {"post.write"}}}
	if err := em.EmitRBAC(em.RBACDef); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "internal", "pkg", "rbac", "rbac.go")); err != nil {
		t.Fatalf("declared roles generate the package: %v", err)
	}
	if err := em.EmitHTTPCommon(nil); err != nil {
		t.Fatal(err)
	}
	common, _ = os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", "common.go"))
	if !strings.Contains(string(common), "rbac.CheckPermission(role, perm)") {
		t.Fatal("with a role table RequirePermission consults it")
	}
}
