package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitFrontendSDK_StripsAuthInjectedFieldsFromPublicArtifacts(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.137"

	services := []ir.Service{
		{
			Name: "Chat",
			Methods: []ir.Method{
				{
					Name: "ListAllChatsForTheCurrentUser",
					Input: &ir.Entity{
						Name: "ListAllChatsForTheCurrentUserRequest",
						Fields: []ir.Field{
							{Name: "companyId", Type: ir.TypeRef{Kind: ir.KindString}},
							{Name: "userId", Type: ir.TypeRef{Kind: ir.KindString}},
							{Name: "limit", Type: ir.TypeRef{Kind: ir.KindInt}},
							{Name: "offset", Type: ir.TypeRef{Kind: ir.KindInt}},
						},
					},
					Output: &ir.Entity{Name: "ListAllChatsForTheCurrentUserResponse"},
				},
				{
					Name: "PostANewMessageToAChat",
					Input: &ir.Entity{
						Name: "PostANewMessageToAChatRequest",
						Fields: []ir.Field{
							{Name: "companyId", Type: ir.TypeRef{Kind: ir.KindString}},
							{Name: "userId", Type: ir.TypeRef{Kind: ir.KindString}},
							{Name: "locale", Type: ir.TypeRef{Kind: ir.KindString}, Optional: true},
							{Name: "timezone", Type: ir.TypeRef{Kind: ir.KindString}, Optional: true},
							{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}},
							{Name: "body", Type: ir.TypeRef{Kind: ir.KindString}, Optional: true},
						},
					},
					Output: &ir.Entity{Name: "PostANewMessageToAChatResponse"},
				},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:  "GET",
			Path:    "/api/chat/v1/all_chats",
			Service: "Chat",
			RPC:     "ListAllChatsForTheCurrentUser",
			Auth:    &ir.EndpointAuth{Type: "jwt", Inject: []string{"companyId", "userId"}},
		},
		{
			Method:  "POST",
			Path:    "/api/chat/v1/chats/{id}/messages",
			Service: "Chat",
			RPC:     "PostANewMessageToAChat",
			Auth:    &ir.EndpointAuth{Type: "jwt", Inject: []string{"companyId", "userId", "locale", "timezone"}},
		},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}
	if err := em.EmitFrontendComponents(services, endpoints, nil); err != nil {
		t.Fatalf("emit frontend components: %v", err)
	}

	typesData, err := os.ReadFile(filepath.Join(tmp, "types", "index.ts"))
	if err != nil {
		t.Fatalf("read generated types: %v", err)
	}
	typesText := string(typesData)
	for _, forbidden := range []string{
		"export interface ListAllChatsForTheCurrentUserRequest {\n  /**  */\n  companyId: string;",
		"export interface ListAllChatsForTheCurrentUserRequest {\n  /**  */\n  userId: string;",
		"export interface PostANewMessageToAChatRequest {\n  /**  */\n  companyId: string;",
		"export interface PostANewMessageToAChatRequest {\n  /**  */\n  userId: string;",
		"export interface PostANewMessageToAChatRequest {\n  /**  */\n  locale?: string;",
		"export interface PostANewMessageToAChatRequest {\n  /**  */\n  timezone?: string;",
	} {
		if strings.Contains(typesText, forbidden) {
			t.Fatalf("expected injected field to be removed from generated types, found %q in:\n%s", forbidden, typesText)
		}
	}
	for _, required := range []string{
		"export interface ListAllChatsForTheCurrentUserRequest {\n  /**  */\n  limit: number;",
		"export interface PostANewMessageToAChatRequest {\n  /**  */\n  id: string;",
	} {
		if !strings.Contains(typesText, required) {
			t.Fatalf("expected generated types to keep non-injected field %q, got:\n%s", required, typesText)
		}
	}

	formData, err := os.ReadFile(filepath.Join(tmp, "components", "forms", "PostANewMessageToAChatForm.schema.ts"))
	if err != nil {
		t.Fatalf("read generated form schema: %v", err)
	}
	formText := string(formData)
	for _, forbidden := range []string{"name: \"companyId\"", "name: \"userId\"", "name: \"locale\"", "name: \"timezone\""} {
		if strings.Contains(formText, forbidden) {
			t.Fatalf("expected injected field to be removed from generated form schema, found %q in:\n%s", forbidden, formText)
		}
	}
	for _, required := range []string{"name: \"id\"", "name: \"body\""} {
		if !strings.Contains(formText, required) {
			t.Fatalf("expected generated form schema to keep field %q, got:\n%s", required, formText)
		}
	}

	manifestData, err := os.ReadFile(filepath.Join(tmp, "sdk-manifest.json"))
	if err != nil {
		t.Fatalf("read generated sdk manifest: %v", err)
	}
	manifestText := string(manifestData)
	if !strings.Contains(manifestText, `"authInject":["companyId","userId","locale","timezone"]`) {
		t.Fatalf("expected generated sdk manifest to expose authInject metadata, got:\n%s", manifestText)
	}
}

func TestEmitFrontendSDK_StripsAuthInjectedFieldsFromNamedRequestEntity(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.139"

	services := []ir.Service{
		{
			Name: "Notifications",
			Methods: []ir.Method{
				{
					Name: "MarkAllNotificationsRead",
					Input: &ir.Entity{
						Name: "MarkAllNotificationsReadRequest",
						Fields: []ir.Field{
							{Name: "userId", Type: ir.TypeRef{Kind: ir.KindString}},
						},
					},
					Output: &ir.Entity{Name: "MarkAllNotificationsReadResponse"},
				},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:  "POST",
			Path:    "/api/notifications/read-all",
			Service: "Notifications",
			RPC:     "MarkAllNotificationsRead",
			Auth:    &ir.EndpointAuth{Type: "jwt", Inject: []string{"userId"}},
		},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	typesText, err := os.ReadFile(filepath.Join(tmp, "types", "index.ts"))
	if err != nil {
		t.Fatalf("read generated types: %v", err)
	}
	text := string(typesText)
	if !strings.Contains(text, "export interface MarkAllNotificationsReadRequest {\n}") {
		t.Fatalf("expected empty MarkAllNotificationsReadRequest after authInject filtering, got:\n%s", text)
	}
	if strings.Contains(text, "userId") {
		t.Fatalf("expected auth-injected userId to be stripped from generated types, got:\n%s", text)
	}
}
