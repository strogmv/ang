package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_GeneratesResilientWebSocketClient(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	services := []ir.Service{
		{
			Name: "Notifications",
			Methods: []ir.Method{
				{
					Name:   "ListNotifications",
					Input:  &ir.Entity{Name: "ListNotificationsRequest"},
					Output: &ir.Entity{Name: "ListNotificationsResponse"},
				},
				{
					Name:   "StreamNotifications",
					Input:  &ir.Entity{Name: "StreamNotificationsRequest"},
					Output: &ir.Entity{Name: "StreamNotificationsResponse"},
				},
			},
		},
		{
			Name: "Tender",
			Methods: []ir.Method{
				{
					Name:   "StreamTenderUpdates",
					Input:  &ir.Entity{Name: "StreamTenderUpdatesRequest"},
					Output: &ir.Entity{Name: "StreamTenderUpdatesResponse"},
				},
			},
		},
		{
			Name: "Chat",
			Methods: []ir.Method{
				{
					Name:   "StreamAppEvents",
					Input:  &ir.Entity{Name: "StreamAppEventsRequest"},
					Output: &ir.Entity{Name: "StreamAppEventsResponse"},
				},
				{
					Name:   "StreamUserChats",
					Input:  &ir.Entity{Name: "StreamUserChatsRequest"},
					Output: &ir.Entity{Name: "StreamUserChatsResponse"},
				},
				{
					Name:   "StreamChatMessages",
					Input:  &ir.Entity{Name: "StreamChatMessagesRequest"},
					Output: &ir.Entity{Name: "StreamChatMessagesResponse"},
				},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:  "GET",
			Path:    "/api/notifications",
			Service: "Notifications",
			RPC:     "ListNotifications",
		},
		{
			Method:   "WS",
			Path:     "/ws/notifications",
			Service:  "Notifications",
			RPC:      "StreamNotifications",
			Messages: []string{"NotificationCreated"},
		},
		{
			Method:   "WS",
			Path:     "/ws/tenders/{tenderId}",
			Service:  "Tender",
			RPC:      "StreamTenderUpdates",
			Messages: []string{"BidPlaced"},
		},
		{
			Method:   "WS",
			Path:     "/ws/app",
			Service:  "Chat",
			RPC:      "StreamAppEvents",
			Messages: []string{"ChatListUpdated", "NotificationCreated", "NotificationRead", "NotificationDismissed", "UserOnlineStatus"},
		},
		{
			Method:   "WS",
			Path:     "/ws/chats",
			Service:  "Chat",
			RPC:      "StreamUserChats",
			Messages: []string{"ChatUpdated"},
		},
		{
			Method:   "WS",
			Path:     "/ws/chats/{chatId}",
			Service:  "Chat",
			RPC:      "StreamChatMessages",
			Messages: []string{"MessageSent"},
		},
	}
	events := []ir.Event{
		{Name: "NotificationCreated"},
		{Name: "BidPlaced"},
		{Name: "ChatUpdated"},
		{Name: "MessageSent"},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, events, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	websocketPath := filepath.Join(tmp, "websocket.ts")
	websocketData, err := os.ReadFile(websocketPath)
	if err != nil {
		t.Fatalf("read generated websocket.ts: %v", err)
	}
	websocketText := string(websocketData)

	mustContain := []string{
		"export const setWebSocketLogger = (logger: WebSocketLogger) => {",
		"export type ConnectState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';",
		"private reconnectTimer: ReturnType<typeof setTimeout> | null = null;",
		"private heartbeatTimer: ReturnType<typeof setInterval> | null = null;",
		"private heartbeatTimeoutTimer: ReturnType<typeof setTimeout> | null = null;",
		"private pendingMessages: any[] = [];",
		"private subscribedRooms: Set<string> = new Set();",
		"public getState(): ConnectState {",
		"public onStateChange(listener: StateListener): () => void {",
		"public setToken(token: string | null) {",
		"public subscribeRoom(room: string) {",
		"public unsubscribeRoom(room: string) {",
		"public setSubscribedRooms(rooms: readonly string[]) {",
		"private flushSubscribedRooms() {",
		"private scheduleReconnect() {",
		"private startHeartbeat() {",
		"private scheduleHeartbeatTimeout() {",
		"if (message?.type === 'pong') {",
		"window.addEventListener('online', this.handleOnline);",
		"document.addEventListener('visibilitychange', this.handleVisibilityChange);",
		"rejectOnce(new Error('WebSocket connection error'));",
		"const delay = Math.min(this.options.reconnectBaseDelayMs * Math.pow(2, this.reconnectAttempts), this.options.maxReconnectDelayMs);",
		"this.pendingMessages.push(data);",
		"this.rawSend({ type: 'subscribe', room: normalized });",
	}
	for _, expected := range mustContain {
		if !strings.Contains(websocketText, expected) {
			t.Fatalf("expected %q in websocket.ts, got:\n%s", expected, websocketText)
		}
	}
	if strings.Contains(websocketText, "maxReconnectAttempts") {
		t.Fatalf("did not expect hard reconnect cap in websocket.ts, got:\n%s", websocketText)
	}
	if strings.Contains(websocketText, "console.error(") || strings.Contains(websocketText, "console.warn(") || strings.Contains(websocketText, "console.log(") {
		t.Fatalf("did not expect direct console usage in websocket.ts, got:\n%s", websocketText)
	}

	hooksPath := filepath.Join(tmp, "hooks", "websocket-hooks.ts")
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read generated websocket-hooks.ts: %v", err)
	}
	hooksText := string(hooksData)
	for _, expected := range []string{
		"import { useQueryClient, type QueryClient } from '@tanstack/react-query';",
		"export type StreamAppEventsWSMessage =",
		"export const streamAppEventsWsClient = new WebSocketClient<StreamAppEventsWSMessage>(",
		"export type AppWSMessage = StreamAppEventsWSMessage;",
		"export const appWsClient = streamAppEventsWsClient;",
		"export const streamNotificationsWsClient = new WebSocketClient<StreamNotificationsWSMessage>(",
		"export const streamUserChatsWsClient = new WebSocketClient<StreamUserChatsWSMessage>(",
		"endpoints.streamAppEventsUrl()",
		"endpoints.streamNotificationsUrl()",
		"endpoints.streamUserChatsUrl()",
		"const applyHotEventCacheUpdate = (",
		"const eventTimestamp = (payload: unknown) => {",
		"applyHotEventCacheUpdate(queryClient, data)",
		"export const subscribeStreamUserChatsRoom = (room: string) => streamUserChatsWsClient.subscribeRoom(room);",
		"export const unsubscribeStreamUserChatsRoom = (room: string) => streamUserChatsWsClient.unsubscribeRoom(room);",
		"export const setStreamUserChatsRooms = (rooms: readonly string[]) => streamUserChatsWsClient.setSubscribedRooms(rooms);",
		"export const createStreamTenderUpdatesWsClient = (",
		"export const createStreamChatMessagesWsClient = (",
		"endpoints.streamTenderUpdatesUrl(params)",
		"endpoints.streamChatMessagesUrl(params)",
		"export const defaultWsClients = [",
		"streamAppEventsWsClient,",
		"export const useStreamTenderUpdatesSubscription = <K extends StreamTenderUpdatesWSMessage['type']>(",
		"export const useStreamChatMessagesSubscription = <K extends StreamChatMessagesWSMessage['type']>(",
		"export const useAppWebsocketSubscription = <K extends AppWSMessage['type']>(",
		"const emit = onMessage as (payload: Extract<T, { type: K }>['payload']) => void;",
		"emit(matched.payload as never);",
		"const emit = onMessageRef.current as unknown as (payload: Extract<AppWSMessage, { type: K }>['payload']) => void;",
		"emit(payload as never);",
		"const emit = onMessageRef.current as unknown as (payload: Extract<StreamChatMessagesWSMessage, { type: K }>['payload']) => void;",
		"emit(matched.payload as never);",
	} {
		if !strings.Contains(hooksText, expected) {
			t.Fatalf("expected %q in websocket-hooks.ts, got:\n%s", expected, hooksText)
		}
	}
	for _, unexpected := range []string{
		"streamNotificationsWsClient,\n  streamUserChatsWsClient,",
		"streamOnlineStatusWsClient,\n] as const;",
	} {
		if strings.Contains(hooksText, unexpected) {
			t.Fatalf("did not expect %q in websocket-hooks.ts, got:\n%s", unexpected, hooksText)
		}
	}
	if strings.Contains(hooksText, "getWebSocketBaseUrl() + '/ws'") {
		t.Fatalf("did not expect websocket-hooks.ts to hardcode a fake /ws singleton, got:\n%s", hooksText)
	}
	if strings.Contains(hooksText, "useStreamUserChatsRooms") {
		t.Fatalf("did not expect shared-room lifecycle hook for singleton websocket client, got:\n%s", hooksText)
	}

	chatEndpointsData, err := os.ReadFile(filepath.Join(tmp, "endpoints", "chat.ts"))
	if err != nil {
		t.Fatalf("read generated endpoints/chat.ts: %v", err)
	}
	notificationEndpointsData, err := os.ReadFile(filepath.Join(tmp, "endpoints", "notifications.ts"))
	if err != nil {
		t.Fatalf("read generated endpoints/notifications.ts: %v", err)
	}
	tenderEndpointsData, err := os.ReadFile(filepath.Join(tmp, "endpoints", "tender.ts"))
	if err != nil {
		t.Fatalf("read generated endpoints/tender.ts: %v", err)
	}
	endpointsText := string(chatEndpointsData) + "\n" + string(notificationEndpointsData) + "\n" + string(tenderEndpointsData)
	for _, expected := range []string{
		"export const streamAppEventsUrl = (",
		"export const streamNotificationsUrl = (",
		"export const streamTenderUpdatesUrl = (",
		"export const streamUserChatsUrl = (",
		"export const streamChatMessagesUrl = (",
		"let streamUrl = resolveStreamUrl(`/ws/app`);",
		"let streamUrl = resolveStreamUrl(`/ws/tenders/${tenderId}`);",
		"let streamUrl = resolveStreamUrl(`/ws/chats/${chatId}`);",
	} {
		if !strings.Contains(endpointsText, expected) {
			t.Fatalf("expected %q in split endpoint modules, got:\n%s", expected, endpointsText)
		}
	}
}
