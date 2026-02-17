package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestPolicyParity_BackendMiddlewareAndSDKMeta(t *testing.T) {
	t.Parallel()

	epNorm := normalizer.Endpoint{
		Method:      "POST",
		Path:        "/api/company/update",
		ServiceName: "Company",
		RPC:         "UpdateCompany",
		AuthType:    "jwt",
		AuthRoles:   []string{"owner", "admin"},
		CacheTTL:    "24h",
		Timeout:     "30s",
		Idempotency: true,
		MaxBodySize: 1024,
		RateLimit: &normalizer.RateLimitDef{
			RPS:   25,
			Burst: 50,
		},
	}
	mw := buildMiddlewareList(epNorm, true, true)
	for _, expected := range []string{
		"AuthMiddleware",
		`RequireRoles([]string{"owner", "admin"})`,
		`CacheMiddleware("24h")`,
		"RateLimitMiddleware(25, 50)",
		`TimeoutMiddleware("30s")`,
		"IdempotencyMiddleware()",
	} {
		if !strings.Contains(mw, expected) {
			t.Fatalf("backend middleware missing %q in %q", expected, mw)
		}
	}

	tmp := t.TempDir()
	em := New(tmp, tmp, "templates")
	em.Version = "0.1.0"
	entities := []ir.Entity{{Name: "Company", Fields: []ir.Field{{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}}}}}
	services := []ir.Service{{Name: "Company", Methods: []ir.Method{{Name: "UpdateCompany", Input: &ir.Entity{Name: "UpdateCompanyRequest"}, Output: &ir.Entity{Name: "UpdateCompanyResponse"}}}}}
	endpoints := []ir.Endpoint{{
		Method:     "POST",
		Path:       "/api/company/update",
		Service:    "Company",
		RPC:        "UpdateCompany",
		Idempotent: true,
		Timeout:    "30s",
		Cache:      "24h",
		RateLimit: &ir.RateLimit{
			RPS:   25,
			Burst: 50,
		},
		Auth:       &ir.EndpointAuth{Type: "jwt", Roles: []string{"owner", "admin"}},
	}}
	if err := em.EmitOpenAPI(endpoints, services, nil, nil); err != nil {
		t.Fatalf("emit openapi: %v", err)
	}
	openapiData, err := os.ReadFile(filepath.Join(tmp, "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	openapiText := string(openapiData)
	for _, expected := range []string{
		"x-idempotency: true",
		"x-timeout: 30s",
		"x-cache-ttl: 24h",
		"x-auth-roles:",
		"- owner",
		"- admin",
		"x-rate-limit:",
		"rps: 25",
		"burst: 50",
	} {
		if !strings.Contains(openapiText, expected) {
			t.Fatalf("openapi policy metadata missing %q", expected)
		}
	}
	if err := em.EmitFrontendSDK(entities, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "endpoints.ts"))
	if err != nil {
		t.Fatalf("read endpoints.ts: %v", err)
	}
	text := string(data)
	for _, expected := range []string{
		"idempotent: true",
		"timeout: '30s'",
		"authRoles: ['owner', 'admin']",
		"cacheTTL: '24h'",
		"rateLimit: {",
		"rps: 25",
		"burst: 50",
		"requiredHeaders: ['Authorization', 'Idempotency-Key']",
		"retryStrategy: {",
		"maxAttempts: 3",
		"retryOnStatuses: [429, 502, 503, 504]",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("sdk endpointMeta missing %q", expected)
		}
	}
}
