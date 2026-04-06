package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_InfiniteQueriesUseUnknownBridgeForRequestCast(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	services := []ir.Service{
		{
			Name: "Tender",
			Methods: []ir.Method{
				{Name: "ListApplications", Input: &ir.Entity{Name: "ListApplicationsRequest"}, Output: &ir.Entity{Name: "ListApplicationsResponse"}},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{Method: "GET", Path: "/api/tenders/{tenderId}/applications", Service: "Tender", RPC: "ListApplications", Pagination: &ir.Pagination{Type: "offset", DefaultLimit: 20}},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	text, err := os.ReadFile(filepath.Join(tmp, "queries", "index.ts"))
	if err != nil {
		t.Fatalf("read queries/index.ts: %v", err)
	}
	out := string(text)
	if !strings.Contains(out, "return api.listApplications(p as unknown as Types.ListApplicationsRequest, { signal });") {
		t.Fatalf("expected unknown bridge request cast in queries/index.ts, got:\n%s", out)
	}
	if strings.Contains(out, "return api.listApplications(p as Types.ListApplicationsRequest, { signal });") {
		t.Fatalf("did not expect direct Record<string, unknown> to request cast in queries/index.ts, got:\n%s", out)
	}
}
