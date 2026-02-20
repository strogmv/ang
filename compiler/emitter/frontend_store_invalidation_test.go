package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/ir"
)

func TestEmitFrontendSDK_GeneratesStoreAutoInvalidation(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	entities := []ir.Entity{
		{
			Name: "Tender",
			Fields: []ir.Field{
				{
					Name: "ID",
					Type: ir.TypeRef{Kind: ir.KindString},
				},
				{
					Name: "Title",
					Type: ir.TypeRef{Kind: ir.KindString},
				},
			},
		},
	}
	services := []ir.Service{
		{
			Name: "Tender",
			Methods: []ir.Method{
				{
					Name:  "CreateTender",
					Input: &ir.Entity{Name: "CreateTenderRequest"},
					Output: &ir.Entity{
						Name: "CreateTenderResponse",
					},
				},
				{
					Name:  "ListTenders",
					Input: &ir.Entity{Name: "ListTendersRequest"},
					Output: &ir.Entity{
						Name: "ListTendersResponse",
					},
				},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:  "GET",
			Path:    "/api/tenders",
			Service: "Tender",
			RPC:     "ListTenders",
		},
		{
			Method:     "POST",
			Path:       "/api/tenders",
			Service:    "Tender",
			RPC:        "CreateTender",
			Invalidate: []string{"ListTenders"},
		},
	}

	if err := em.EmitFrontendSDK(entities, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	// stores/invalidation.ts should be an empty stub — TanStack Query owns cache invalidation.
	invalidationData, err := os.ReadFile(filepath.Join(tmp, "stores", "invalidation.ts"))
	if err != nil {
		t.Fatalf("read stores/invalidation.ts: %v", err)
	}
	if strings.Contains(string(invalidationData), "markStoresListStale") {
		t.Fatalf("stores/invalidation.ts must not export markStoresListStale (Zustand invalidation removed)")
	}

	// No entity store files should be generated (Zustand stores removed).
	if _, err := os.Stat(filepath.Join(tmp, "stores", "tender.ts")); !os.IsNotExist(err) {
		t.Fatalf("stores/tender.ts must not be generated (entity stores removed)")
	}

	// endpoints.ts should NOT reference the old Zustand invalidation helpers.
	endpointsData, err := os.ReadFile(filepath.Join(tmp, "endpoints.ts"))
	if err != nil {
		t.Fatalf("read endpoints.ts: %v", err)
	}
	endpointsText := string(endpointsData)
	if strings.Contains(endpointsText, "markInvalidateTargets") {
		t.Fatalf("endpoints.ts must not call markInvalidateTargets (Zustand invalidation removed)")
	}

	// hooks/index.ts should contain TanStack Query invalidation for CreateTender → ListTenders.
	hooksData, err := os.ReadFile(filepath.Join(tmp, "hooks", "index.ts"))
	if err != nil {
		t.Fatalf("read hooks/index.ts: %v", err)
	}
	hooksText := string(hooksData)
	if !strings.Contains(hooksText, "invalidateQueries") {
		t.Fatalf("hooks/index.ts should contain invalidateQueries for CreateTender mutation")
	}
}

func TestEmitFrontendSDK_EndpointMetaPolicyParity(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	entities := []ir.Entity{
		{
			Name: "Company",
			Fields: []ir.Field{
				{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}},
			},
		},
	}
	services := []ir.Service{
		{
			Name: "Company",
			Methods: []ir.Method{
				{Name: "UpdateCompany", Input: &ir.Entity{Name: "UpdateCompanyRequest"}, Output: &ir.Entity{Name: "UpdateCompanyResponse"}},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:     "POST",
			Path:       "/api/company/update",
			Service:    "Company",
			RPC:        "UpdateCompany",
			Idempotent: true,
			Timeout:    "30s",
			Cache:      "24h",
			Auth: &ir.EndpointAuth{
				Type:  "jwt",
				Roles: []string{"owner", "admin"},
			},
		},
	}

	if err := em.EmitFrontendSDK(entities, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	endpointsData, err := os.ReadFile(filepath.Join(tmp, "endpoints.ts"))
	if err != nil {
		t.Fatalf("read endpoints.ts: %v", err)
	}
	text := string(endpointsData)
	for _, expected := range []string{
		"idempotent: true",
		"timeout: '30s'",
		"authRoles: ['owner', 'admin']",
		"cacheTTL: '24h'",
		"retryStrategy: {",
		"maxAttempts: 3",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in endpointMeta, got:\n%s", expected, text)
		}
	}
}

func TestEmitFrontendSDK_InvalidateTargetsCarryScopeForDetailRPC(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	entities := []ir.Entity{
		{
			Name: "Tender",
			Fields: []ir.Field{
				{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}},
			},
		},
	}
	services := []ir.Service{
		{
			Name: "Tender",
			Methods: []ir.Method{
				{Name: "GetTender", Input: &ir.Entity{Name: "GetTenderRequest"}, Output: &ir.Entity{Name: "GetTenderResponse"}},
				{Name: "UpdateTender", Input: &ir.Entity{Name: "UpdateTenderRequest"}, Output: &ir.Entity{Name: "UpdateTenderResponse"}},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:  "GET",
			Path:    "/api/tenders/{tenderId}",
			Service: "Tender",
			RPC:     "GetTender",
		},
		{
			Method:     "PATCH",
			Path:       "/api/tenders/{tenderId}",
			Service:    "Tender",
			RPC:        "UpdateTender",
			Invalidate: []string{"GetTender"},
		},
	}

	if err := em.EmitFrontendSDK(entities, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	endpointsData, err := os.ReadFile(filepath.Join(tmp, "endpoints.ts"))
	if err != nil {
		t.Fatalf("read endpoints.ts: %v", err)
	}
	text := string(endpointsData)
	if !strings.Contains(text, "scopeParam: 'tenderId'") || !strings.Contains(text, "mode: 'detail'") {
		t.Fatalf("expected scoped detail invalidation target in endpointMeta, got:\n%s", text)
	}
}
