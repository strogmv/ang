package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// A Telegram webhook secret travelled in the path (dealingi-back: the access
// log, proxies and Telegram's own dashboards kept it); Telegram sends it as
// X-Telegram-Bot-Api-Secret-Token when the webhook is registered with
// secret_token. @header binds an input field to a request header: the
// handler reads it from there only, OpenAPI lists it, the SDK sends it.
func TestHeaderInputsAreReadFromTheHeaderOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.GoModule = "example.com/shop"
	str := ir.TypeRef{Kind: ir.KindString}
	header := map[string]any{normalizer.FieldMetaHeader: "X-Telegram-Bot-Api-Secret-Token"}
	schema := &ir.Schema{
		Services: []ir.Service{{
			Name: "Bots",
			Methods: []ir.Method{
				{
					Name: "TelegramWebhook",
					Input: &ir.Entity{Name: "TelegramWebhookRequest", Fields: []ir.Field{
						{Name: "secretToken", Type: str, Metadata: header},
						{Name: "updateId", Type: ir.TypeRef{Kind: ir.KindInt}, Optional: true},
					}},
					Output: &ir.Entity{Name: "TelegramWebhookResponse", Fields: []ir.Field{{Name: "ok", Type: ir.TypeRef{Kind: ir.KindBool}}}},
				},
				{
					Name: "ProbeWebhook",
					Input: &ir.Entity{Name: "ProbeWebhookRequest", Fields: []ir.Field{
						{Name: "secretToken", Type: str, Metadata: header},
					}},
					Output: &ir.Entity{Name: "ProbeWebhookResponse", Fields: []ir.Field{{Name: "ok", Type: ir.TypeRef{Kind: ir.KindBool}}}},
				},
			},
		}},
		Endpoints: []ir.Endpoint{
			{Method: "POST", Path: "/telegram/webhook", Service: "Bots", RPC: "TelegramWebhook"},
			{Method: "GET", Path: "/telegram/webhook", Service: "Bots", RPC: "ProbeWebhook"},
		},
	}
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitHTTPFromIR(schema, nil); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitOpenAPI(schema.Endpoints, schema.Services, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitFrontendSDK(nil, schema.Services, schema.Endpoints, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	read := func(rel ...string) string {
		data, err := os.ReadFile(filepath.Join(rel...))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	normalize := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	handlers := read(root, "internal", "transport", "http", "bots.go")
	if n := strings.Count(handlers, `req.SecretToken = r.Header.Get("X-Telegram-Bot-Api-Secret-Token")`); n != 2 {
		t.Fatalf("both handlers read the header (%d):\n%s", n, handlers)
	}
	if strings.Contains(handlers, `r.URL.Query().Get("secretToken")`) {
		t.Fatalf("a header field is never read from the query:\n%s", handlers)
	}
	post := handlers[strings.Index(handlers, "func handlerTelegramWebhook("):]
	if strings.Index(post, "r.Header.Get(") < strings.Index(post, "decodeJSONRequest(") {
		t.Fatal("the header is read after the body, so a body value cannot stand in for it")
	}
	openapi := normalize(read(root, "api", "openapi.yaml"))
	if !strings.Contains(openapi, normalize("- in: header name: X-Telegram-Bot-Api-Secret-Token required: true schema: type: string")) {
		t.Fatalf("OpenAPI lacks the header parameter:\n%s", read(root, "api", "openapi.yaml"))
	}
	sdk := normalize(read(root, "sdk", "endpoints", "bots.ts"))
	for _, want := range []string{
		`if (params.secretToken != null) headers["X-Telegram-Bot-Api-Secret-Token"] = String(params.secretToken);`,
		"omitKeys(toParamRecord(params), ['secretToken']), { headers, signal: requestOptions.signal }",
		"omitKeys(toParamRecord(params), ['secretToken']); return apiClient.get",
		"{ params: queryParams, headers, signal: requestOptions.signal }",
	} {
		if !strings.Contains(sdk, normalize(want)) {
			t.Errorf("SDK lacks %s:\n%s", want, read(root, "sdk", "endpoints", "bots.ts"))
		}
	}
}
