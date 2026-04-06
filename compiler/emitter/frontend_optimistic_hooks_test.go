package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_GeneratesOptimisticHooksWithObjectConstraint(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	services := []ir.Service{
		{
			Name: "Notification",
			Methods: []ir.Method{
				{Name: "ListNotifications", Input: &ir.Entity{Name: "ListNotificationsRequest"}, Output: &ir.Entity{Name: "ListNotificationsResponse"}},
				{Name: "GetNotification", Input: &ir.Entity{Name: "GetNotificationRequest"}, Output: &ir.Entity{Name: "GetNotificationResponse"}},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{Method: "GET", Path: "/api/notifications", Service: "Notification", RPC: "ListNotifications"},
		{Method: "GET", Path: "/api/notifications/{id}", Service: "Notification", RPC: "GetNotification"},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	text, err := os.ReadFile(filepath.Join(tmp, "hooks", "optimistic-hooks.ts"))
	if err != nil {
		t.Fatalf("read hooks/optimistic-hooks.ts: %v", err)
	}
	out := string(text)
	for _, expected := range []string{
		"useOptimisticCreateNotifications = <T extends object & Partial<Types.GetNotificationResponse>, TReq = unknown>(",
		"useOptimisticUpdateNotifications = <T extends object & Partial<Types.GetNotificationResponse>, TReq = unknown>(",
		"return Array.isArray(old) ? updated : { ...((old as Record<string, unknown>) ?? {}), data: updated };",
		"store.markListStale();",
		"store.markDetailStale(id as Parameters<typeof store.markDetailStale>[0]);",
	} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected %q in hooks/optimistic-hooks.ts, got:\n%s", expected, out)
		}
	}
}
