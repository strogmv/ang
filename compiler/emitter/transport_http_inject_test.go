package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitHTTP_AuthInjectOverridesQueryParams(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	schema := &ir.Schema{
		Services: []ir.Service{
			{
				Name: "Chat",
				Methods: []ir.Method{
					{
						Name: "ListAllChatsForTheCurrentUser",
						Input: &ir.Entity{
							Name: "ListAllChatsForTheCurrentUserRequest",
							Fields: []ir.Field{
								{Name: "companyId", Type: ir.TypeRef{Kind: ir.KindString}},
								{Name: "userId", Type: ir.TypeRef{Kind: ir.KindString}},
								{Name: "limit", Type: ir.TypeRef{Kind: ir.KindInt}},
							},
						},
						Output: &ir.Entity{Name: "ListAllChatsForTheCurrentUserResponse"},
					},
				},
			},
		},
		Endpoints: []ir.Endpoint{
			{
				Method:  "GET",
				Path:    "/api/chat/v1/all_chats",
				Service: "Chat",
				RPC:     "ListAllChatsForTheCurrentUser",
				Auth: &ir.EndpointAuth{
					Type:   "jwt",
					Inject: []string{"companyId", "userId"},
				},
			},
		},
	}

	em := New(root, filepath.Join(root, "sdk"), "templates")
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatalf("EmitServiceFromIR failed: %v", err)
	}
	if err := em.EmitHTTPFromIR(schema, nil); err != nil {
		t.Fatalf("EmitHTTPFromIR failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "internal", "transport", "http", "chat.go"))
	if err != nil {
		t.Fatalf("read generated HTTP file: %v", err)
	}
	src := string(data)

	if strings.Contains(src, `if req.CompanyID == ""`) {
		t.Fatalf("expected auth-injected CompanyID to override query params, got fallback-only code:\n%s", src)
	}
	if strings.Contains(src, `if req.UserID == ""`) {
		t.Fatalf("expected auth-injected UserID to override query params, got fallback-only code:\n%s", src)
	}
	for _, needle := range []string{
		`req.CompanyID = CurrentCompanyID(r)`,
		`req.UserID = CurrentUserID(r)`,
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("expected generated HTTP handler to contain %q, got:\n%s", needle, src)
		}
	}
	for _, forbidden := range []string{
		`r.URL.Query().Get("companyid")`,
		`r.URL.Query().Get("userid")`,
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("expected generated HTTP handler to ignore query overrides for injected auth fields, but found %q in:\n%s", forbidden, src)
		}
	}
}
