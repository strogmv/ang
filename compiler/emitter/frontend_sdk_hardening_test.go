package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_GeneratesHardenedClientAndEndpoints(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

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
			Metadata: map[string]any{
				"frontend": map[string]any{
					"queryProfile": "admin",
				},
			},
		},
		{
			Method:  "PATCH",
			Path:    "/api/tenders/{tenderId}",
			Service: "Tender",
			RPC:     "UpdateTender",
			Metadata: map[string]any{
				"frontend": map[string]any{
					"invalidateTargets": []map[string]any{
						{"store": "tender", "scopeParam": "tenderId", "mode": "detail"},
					},
				},
			},
		},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	apiClientText, err := os.ReadFile(filepath.Join(tmp, "api-client.ts"))
	if err != nil {
		t.Fatalf("read api-client.ts: %v", err)
	}
	client := string(apiClientText)
	for _, expected := range []string{
		"export type ApiRequestOptions = {",
		"setApiClientLogger",
		"setResponseValidationReporter",
		"Secure random generation is unavailable in this environment",
		"type RefreshWaiter = {",
		"let refreshPromise: Promise<string> | null = null;",
		"waitForRefreshToken() : await refreshAuthToken()",
		"if (shouldClearAuthOnRefreshFailure(err)) {",
		"responseValidationReporter(issue)",
		"apiLogger.error('[ANG SDK] API request failed'",
	} {
		if !strings.Contains(client, expected) {
			t.Fatalf("expected %q in api-client.ts, got:\n%s", expected, client)
		}
	}
	if strings.Contains(client, "Math.random()") {
		t.Fatalf("did not expect Math.random fallback in api-client.ts, got:\n%s", client)
	}
	if strings.Contains(client, "console.error(") || strings.Contains(client, "console.warn(") {
		t.Fatalf("did not expect direct console usage in api-client.ts, got:\n%s", client)
	}

	endpointsText, err := os.ReadFile(filepath.Join(tmp, "endpoints.ts"))
	if err != nil {
		t.Fatalf("read endpoints.ts: %v", err)
	}
	e := string(endpointsText)
	for _, expected := range []string{
		"type ApiRequestOptions",
		"requestOptions: ApiRequestOptions = {}",
		"const paramsRecord = toParamRecord(params);",
		"getPathParamValue(paramsRecord, 'tenderId'",
		"Missing required path param: ${primary}",
		"signal: requestOptions.signal",
		"const queryParams = omitKeys(toParamRecord(params)",
	} {
		if !strings.Contains(e, expected) {
			t.Fatalf("expected %q in endpoints.ts, got:\n%s", expected, e)
		}
	}
	if strings.Contains(e, "// @ts-ignore") {
		t.Fatalf("did not expect ts-ignore path param extraction in endpoints.ts, got:\n%s", e)
	}

	queryOptionsText, err := os.ReadFile(filepath.Join(tmp, "query-options.ts"))
	if err != nil {
		t.Fatalf("read query-options.ts: %v", err)
	}
	q := string(queryOptionsText)
	for _, expected := range []string{
		"fresh:",
		"standard:",
		"admin:",
		"queryFn: ({ signal }) => api.getTender(params, { signal })",
		"...(queryProfiles['admin'] || {})",
	} {
		if !strings.Contains(q, expected) {
			t.Fatalf("expected %q in query-options.ts, got:\n%s", expected, q)
		}
	}

	providersText, err := os.ReadFile(filepath.Join(tmp, "providers.tsx"))
	if err != nil {
		t.Fatalf("read providers.tsx: %v", err)
	}
	p := string(providersText)
	if strings.Contains(p, "@ts-nocheck") {
		t.Fatalf("did not expect ts-nocheck in providers.tsx, got:\n%s", p)
	}
	if !strings.Contains(p, "const defaultQueryProfile = queryProfiles.standard || {}") {
		t.Fatalf("expected providers.tsx to use standard query profile, got:\n%s", p)
	}

	hooksText, err := os.ReadFile(filepath.Join(tmp, "hooks", "index.ts"))
	if err != nil {
		t.Fatalf("read hooks/index.ts: %v", err)
	}
	h := string(hooksText)
	for _, expected := range []string{
		"import { endpointMeta } from '../endpoints';",
		"const invalidateGeneratedTargets = (",
		"service?: string;",
		"rpc?: string;",
		"if (serviceName && rpcName) {",
		"String(key[0] || '') !== serviceName",
		"String(key[1] || '') !== rpcName",
		"invalidateGeneratedTargets(queryClient, 'updateTender', (_variables ?? {}) as Record<string, unknown>);",
		"throw missingRequiredParamsError('useGetTenderSuspense');",
	} {
		if !strings.Contains(h, expected) {
			t.Fatalf("expected %q in hooks/index.ts, got:\n%s", expected, h)
		}
	}
	if strings.Contains(h, "// @ts-ignore") {
		t.Fatalf("did not expect ts-ignore in hooks/index.ts, got:\n%s", h)
	}
}
