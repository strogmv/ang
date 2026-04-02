package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_GeneratesHotEventCachePatches(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	entities := []ir.Entity{
		{Name: "ListNotificationsRequest"},
		{Name: "ListNotificationsResponse", Fields: []ir.Field{{Name: "data", Type: ir.TypeRef{Kind: ir.KindList, ItemType: &ir.TypeRef{Kind: ir.KindEntity, Name: "ListNotificationsResponseData"}}}}},
		{Name: "ListNotificationsResponseData", Fields: []ir.Field{{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "title", Type: ir.TypeRef{Kind: ir.KindString}}}},
		{Name: "ListBidsRequest", Fields: []ir.Field{{Name: "tenderId", Type: ir.TypeRef{Kind: ir.KindString}}}},
		{Name: "ListBidsResponse", Fields: []ir.Field{{Name: "data", Type: ir.TypeRef{Kind: ir.KindList, ItemType: &ir.TypeRef{Kind: ir.KindEntity, Name: "ListBidsResponseData"}}}}},
		{Name: "ListBidsResponseData", Fields: []ir.Field{{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "tenderId", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "amount", Type: ir.TypeRef{Kind: ir.KindFloat}}, {Name: "createdAt", Type: ir.TypeRef{Kind: ir.KindString}}}},
		{Name: "GetMessagesFromAChatRequest", Fields: []ir.Field{{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}}}},
		{Name: "GetMessagesFromAChatResponse", Fields: []ir.Field{{Name: "data", Type: ir.TypeRef{Kind: ir.KindList, ItemType: &ir.TypeRef{Kind: ir.KindEntity, Name: "GetMessagesFromAChatResponseData"}}}, {Name: "total", Type: ir.TypeRef{Kind: ir.KindInt}}}},
		{Name: "GetMessagesFromAChatResponseData", Fields: []ir.Field{{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "chatId", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "senderUserId", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "senderCompanyId", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "contactId", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "value", Type: ir.TypeRef{Kind: ir.KindString}}, {Name: "body", Type: ir.TypeRef{Kind: ir.KindString}, Optional: true}, {Name: "attachmentId", Type: ir.TypeRef{Kind: ir.KindString}, Optional: true}, {Name: "replyToId", Type: ir.TypeRef{Kind: ir.KindString}, Optional: true}, {Name: "createdAt", Type: ir.TypeRef{Kind: ir.KindString}}}},
	}

	services := []ir.Service{
		{Name: "Notifications", Methods: []ir.Method{{Name: "ListNotifications", Input: &ir.Entity{Name: "ListNotificationsRequest"}, Output: &ir.Entity{Name: "ListNotificationsResponse"}}, {Name: "StreamAppEvents", Input: &ir.Entity{Name: "StreamAppEventsRequest"}, Output: &ir.Entity{Name: "StreamAppEventsResponse"}}}},
		{Name: "Bids", Methods: []ir.Method{{Name: "ListBids", Input: &ir.Entity{Name: "ListBidsRequest"}, Output: &ir.Entity{Name: "ListBidsResponse"}}, {Name: "StreamTenderUpdates", Input: &ir.Entity{Name: "StreamTenderUpdatesRequest"}, Output: &ir.Entity{Name: "StreamTenderUpdatesResponse"}}}},
		{Name: "Chat", Methods: []ir.Method{{Name: "GetMessagesFromAChat", Input: &ir.Entity{Name: "GetMessagesFromAChatRequest"}, Output: &ir.Entity{Name: "GetMessagesFromAChatResponse"}}, {Name: "StreamChatMessages", Input: &ir.Entity{Name: "StreamChatMessagesRequest"}, Output: &ir.Entity{Name: "StreamChatMessagesResponse"}}}},
	}

	endpoints := []ir.Endpoint{
		{Method: "GET", Path: "/api/notifications", Service: "Notifications", RPC: "ListNotifications"},
		{Method: "GET", Path: "/api/bids", Service: "Bids", RPC: "ListBids"},
		{Method: "GET", Path: "/api/chats/{ID}/messages", Service: "Chat", RPC: "GetMessagesFromAChat"},
		{Method: "WS", Path: "/ws/app", Service: "Notifications", RPC: "StreamAppEvents", Messages: []string{"NotificationCreated"}},
		{Method: "WS", Path: "/ws/tenders/{tenderId}", Service: "Bids", RPC: "StreamTenderUpdates", Messages: []string{"BidPlaced"}},
		{Method: "WS", Path: "/ws/chats/{chatId}", Service: "Chat", RPC: "StreamChatMessages", Messages: []string{"ChatMessageReceived"}},
	}

	events := []ir.Event{{Name: "NotificationCreated"}, {Name: "BidPlaced"}, {Name: "ChatMessageReceived"}}

	if err := em.EmitFrontendSDK(entities, services, endpoints, events, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	hooksData, err := os.ReadFile(filepath.Join(tmp, "hooks", "websocket-hooks.ts"))
	if err != nil {
		t.Fatalf("read websocket-hooks.ts: %v", err)
	}
	hooksText := string(hooksData)
	for _, expected := range []string{
		"if (data.type === 'NotificationCreated') {",
		"queryClient.setQueriesData<Types.ListNotificationsResponse>(",
		"matchesEndpointQuery(query.queryKey, 'Notifications', 'ListNotifications')",
		"if (data.type === 'BidPlaced') {",
		"queryClient.setQueriesData<Types.ListBidsResponse>(",
		"matchesEndpointQuery(query.queryKey, 'Bids', 'ListBids')",
		"if (data.type === 'ChatMessageReceived') {",
		"queryClient.setQueriesData<Types.GetMessagesFromAChatResponse>(",
		"matchesEndpointQuery(query.queryKey, 'Chat', 'GetMessagesFromAChat')",
		"applyHotEventCacheUpdate(queryClient, data as WSMessage);",
	} {
		if !strings.Contains(hooksText, expected) {
			t.Fatalf("expected %q in websocket-hooks.ts, got\n%s", expected, hooksText)
		}
	}
}
