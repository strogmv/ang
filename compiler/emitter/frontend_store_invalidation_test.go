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
			Method:  "POST",
			Path:    "/api/tenders",
			Service: "Tender",
			RPC:     "CreateTender",
		},
	}

	if err := em.EmitFrontendSDK(entities, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	invalidationData, err := os.ReadFile(filepath.Join(tmp, "stores", "invalidation.ts"))
	if err != nil {
		t.Fatalf("read stores/invalidation.ts: %v", err)
	}
	if !strings.Contains(string(invalidationData), "markStoresListStale") {
		t.Fatalf("expected markStoresListStale in invalidation module")
	}

	storeData, err := os.ReadFile(filepath.Join(tmp, "stores", "tender.ts"))
	if err != nil {
		t.Fatalf("read stores/tender.ts: %v", err)
	}
	storeText := string(storeData)
	if !strings.Contains(storeText, "listStale: boolean;") || !strings.Contains(storeText, "markListStale: () => void;") {
		t.Fatalf("expected stale metadata and markListStale in tender store")
	}
	if !strings.Contains(storeText, "registerStoreInvalidator('tender'") {
		t.Fatalf("expected tender store to register invalidator")
	}

	endpointsData, err := os.ReadFile(filepath.Join(tmp, "endpoints.ts"))
	if err != nil {
		t.Fatalf("read endpoints.ts: %v", err)
	}
	endpointsText := string(endpointsData)
	if !strings.Contains(endpointsText, "import { markStoresListStale } from './stores/invalidation';") {
		t.Fatalf("expected endpoints.ts to import markStoresListStale")
	}
	if !strings.Contains(endpointsText, "invalidateStores: ['tender']") {
		t.Fatalf("expected create endpoint metadata to include tender invalidation")
	}
	if !strings.Contains(endpointsText, "markStoresListStale(storesToInvalidate)") {
		t.Fatalf("expected mutation endpoint to call markStoresListStale")
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
