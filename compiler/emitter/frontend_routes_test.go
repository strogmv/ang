package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_RouteLoadersPassParamsObject(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	services := []ir.Service{
		{
			Name: "User",
			Methods: []ir.Method{
				{Name: "GetPublicUserAvatar", Input: &ir.Entity{Name: "GetPublicUserAvatarRequest"}, Output: &ir.Entity{Name: "GetPublicUserAvatarResponse"}},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:      "GET",
			Path:        "/api/public/user-avatars/{assetId}/{variant}",
			Service:     "User",
			RPC:         "GetPublicUserAvatar",
			Description: "Read owner's \"public\" avatar",
		},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	text, err := os.ReadFile(filepath.Join(tmp, "routes.ts"))
	if err != nil {
		t.Fatalf("read routes.ts: %v", err)
	}
	out := string(text)
	if !strings.Contains(out, "await queryClient.ensureQueryData(Queries.getPublicUserAvatarQueryOptions({assetId: params.assetId, variant: params.variant}));") {
		t.Fatalf("expected object param route loader in routes.ts, got:\n%s", out)
	}
	if strings.Contains(out, "Queries.getPublicUserAvatarQueryOptions(params.assetId, params.variant)") {
		t.Fatalf("did not expect positional route loader args in routes.ts, got:\n%s", out)
	}
	if !strings.Contains(out, `title: "Read owner's \"public\" avatar"`) {
		t.Fatalf("expected escaped TypeScript route title, got:\n%s", out)
	}
}
