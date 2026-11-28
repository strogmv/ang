package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

// A payment provider (Express-Pay) POSTs application/x-www-form-urlencoded to
// a @rawBody() webhook; the handler used to refuse it with 400 before the
// operation, which parses the captured bytes itself, ran. The raw body is
// still captured, but the JSON decode is skipped for a form-encoded request.
func TestRawBodyHandlerSkipsJSONDecodeForFormURLEncoded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.GoModule = "example.com/shop"
	schema := &ir.Schema{
		Services: []ir.Service{{
			Name: "Pay",
			Methods: []ir.Method{
				{
					Name:       "ReceiveNotification",
					Attributes: []ir.Attribute{{Name: "rawBody"}},
					Input: &ir.Entity{Name: "ReceiveNotificationRequest", Fields: []ir.Field{
						{Name: "body", Type: ir.TypeRef{Kind: ir.KindAny}},
					}},
					Output: &ir.Entity{Name: "ReceiveNotificationResponse", Fields: []ir.Field{{Name: "ok", Type: ir.TypeRef{Kind: ir.KindBool}}}},
				},
				{
					Name: "Create",
					Input: &ir.Entity{Name: "CreateRequest", Fields: []ir.Field{
						{Name: "title", Type: ir.TypeRef{Kind: ir.KindString}},
					}},
					Output: &ir.Entity{Name: "CreateResponse", Fields: []ir.Field{{Name: "ok", Type: ir.TypeRef{Kind: ir.KindBool}}}},
				},
			},
		}},
		Endpoints: []ir.Endpoint{
			{Method: "POST", Path: "/pay/notify", Service: "Pay", RPC: "ReceiveNotification"},
			{Method: "POST", Path: "/pay/create", Service: "Pay", RPC: "Create"},
		},
	}
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitHTTPFromIR(schema, nil); err != nil {
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
	handlers := normalize(read(root, "internal", "transport", "http", "pay.go"))
	for _, want := range []string{
		"r = captureRawBody(r) if !isFormURLEncoded(r) { if err := decodeJSONRequest(r, &req.Body); err != nil {",
		"if err := decodeJSONRequest(r, &req); err != nil {", // the ordinary operation is untouched
	} {
		if !strings.Contains(handlers, want) {
			t.Errorf("handler lacks %q:\n%s", want, handlers)
		}
	}
	if n := strings.Count(handlers, "isFormURLEncoded("); n != 1 {
		t.Errorf("only the raw-body handler checks the content type, got %d uses", n)
	}
	common := normalize(read(root, "internal", "transport", "http", "common.go"))
	if !strings.Contains(common, `func isFormURLEncoded(r *http.Request) bool { mediatype, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")) return err == nil && mediatype == "application/x-www-form-urlencoded" }`) {
		t.Errorf("common.go lacks the helper")
	}
}
