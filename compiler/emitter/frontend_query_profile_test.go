package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_UsesEndpointFrontendMetadataProfiles(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	services := []ir.Service{
		{
			Name: "Realtime",
			Methods: []ir.Method{
				{Name: "ListPresenceFeed", Input: &ir.Entity{Name: "ListPresenceFeedRequest"}, Output: &ir.Entity{Name: "ListPresenceFeedResponse"}},
				{Name: "GetPresenceRoom", Input: &ir.Entity{Name: "GetPresenceRoomRequest"}, Output: &ir.Entity{Name: "GetPresenceRoomResponse"}},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:  "GET",
			Path:    "/api/presence/feed",
			Service: "Realtime",
			RPC:     "ListPresenceFeed",
			Metadata: map[string]any{
				"frontend": map[string]any{
					"queryProfile": "realtime",
					"cachePolicy":  "realtime",
				},
			},
		},
		{
			Method:  "GET",
			Path:    "/api/presence/rooms/{id}",
			Service: "Realtime",
			RPC:     "GetPresenceRoom",
			Metadata: map[string]any{
				"frontend": map[string]any{
					"queryProfile": "realtime",
					"cachePolicy":  "realtime",
				},
			},
		},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	queryOptionsText, err := os.ReadFile(filepath.Join(tmp, "query-options.ts"))
	if err != nil {
		t.Fatalf("read query-options.ts: %v", err)
	}
	q := string(queryOptionsText)
	for _, expected := range []string{
		"export const queryProfiles",
		"realtime:",
		"refetchOnMount: 'always'",
		"export const endpointQueryOptions = {",
		"listPresenceFeed: (params: Types.ListPresenceFeedRequest",
		"queryKey: queryKeys.Realtime.ListPresenceFeed(params)",
		"...(queryProfiles['realtime'] || {})",
		"getPresenceRoom: (params: Types.GetPresenceRoomRequest",
	} {
		if !strings.Contains(q, expected) {
			t.Fatalf("expected %q in query-options.ts, got:\n%s", expected, q)
		}
	}

	hooksText, err := os.ReadFile(filepath.Join(tmp, "hooks", "index.ts"))
	if err != nil {
		t.Fatalf("read hooks/index.ts: %v", err)
	}
	h := string(hooksText)
	for _, expected := range []string{
		"...QueryOptions.endpointQueryOptions.listPresenceFeed(params)",
		"...QueryOptions.endpointQueryOptions.getPresenceRoom(params)",
	} {
		if !strings.Contains(h, expected) {
			t.Fatalf("expected %q in hooks/index.ts, got:\n%s", expected, h)
		}
	}

	endpointsText, err := os.ReadFile(filepath.Join(tmp, "endpoints.ts"))
	if err != nil {
		t.Fatalf("read endpoints.ts: %v", err)
	}
	e := string(endpointsText)
	for _, expected := range []string{
		"queryProfile?: string;",
		"cachePolicy?: string;",
		"queryProfile: 'realtime'",
		"cachePolicy: 'realtime'",
	} {
		if !strings.Contains(e, expected) {
			t.Fatalf("expected %q in endpoints.ts, got:\n%s", expected, e)
		}
	}

	clientText, err := os.ReadFile(filepath.Join(tmp, "api-client.ts"))
	if err != nil {
		t.Fatalf("read api-client.ts: %v", err)
	}
	c := string(clientText)
	for _, expected := range []string{
		"meta?.cachePolicy === 'realtime'",
		"_rt: Date.now().toString()",
		"Cache-Control', 'no-store, no-cache, max-age=0, must-revalidate'",
		"config.headers.set('Pragma', 'no-cache')",
	} {
		if !strings.Contains(c, expected) {
			t.Fatalf("expected %q in api-client.ts, got:\n%s", expected, c)
		}
	}
}
