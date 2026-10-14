package emitter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

func fileOutputSchema() *ir.Schema {
	fileFields := []ir.Field{
		{Name: "body", Type: ir.TypeRef{Kind: ir.KindAny}, IsSecret: true, Optional: true},
		{Name: "content", Type: ir.TypeRef{Kind: ir.KindAny}, IsSecret: true, Optional: true},
		{Name: "contentType", Type: ir.TypeRef{Kind: ir.KindString}, IsSecret: true, Optional: true},
		{Name: "filename", Type: ir.TypeRef{Kind: ir.KindString}, IsSecret: true, Optional: true},
		{Name: "size", Type: ir.TypeRef{Kind: ir.KindInt64}, IsSecret: true, Optional: true},
	}
	return &ir.Schema{
		Services: []ir.Service{
			{
				Name: "Reports",
				Methods: []ir.Method{{
					Name:     "ExportOrders",
					Input:    &ir.Entity{Name: "ExportOrdersRequest", Fields: []ir.Field{{Name: "companyId", Type: ir.TypeRef{Kind: ir.KindString}}}},
					Output:   &ir.Entity{Name: "ExportOrdersResponse", Fields: fileFields},
					Metadata: map[string]any{normalizer.MethodMetaOutputFile: map[string]any{"content_type": "text/csv; charset=utf-8", "filename": "orders.csv", "disposition": "attachment"}},
				}},
			},
			{
				Name: "Orders",
				Methods: []ir.Method{
					{
						Name:     "InvoicePDF",
						Input:    &ir.Entity{Name: "InvoicePDFRequest", Fields: []ir.Field{{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}}},
						Output:   &ir.Entity{Name: "InvoicePDFResponse", Fields: fileFields},
						Metadata: map[string]any{normalizer.MethodMetaOutputFile: map[string]any{"content_type": "application/pdf", "disposition": "inline"}},
					},
					{
						Name:   "GetOrder",
						Input:  &ir.Entity{Name: "GetOrderRequest", Fields: []ir.Field{{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}}},
						Output: &ir.Entity{Name: "GetOrderResponse", Fields: []ir.Field{{Name: "total", Type: ir.TypeRef{Kind: ir.KindInt}}}},
					},
				},
			},
		},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/reports/orders.csv", Service: "Reports", RPC: "ExportOrders", Auth: &ir.EndpointAuth{Type: "jwt", Inject: []string{"companyId"}}},
			{Method: "GET", Path: "/orders/{id}/invoice.pdf", Service: "Orders", RPC: "InvoicePDF"},
			{Method: "GET", Path: "/orders/{id}", Service: "Orders", RPC: "GetOrder"},
		},
	}
}

func TestEmitHTTPWritesFileOutputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.GoModule = "example.com/shop"
	schema := fileOutputSchema()
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitHTTPFromIR(schema, nil); err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	port := strings.Join(strings.Fields(read("internal/port/reports.go")), " ")
	for _, want := range []string{`"io"`, "Body io.ReadCloser", "Content []byte", "Size int64"} {
		if !strings.Contains(port, want) {
			t.Fatalf("port lacks %q:\n%s", want, port)
		}
	}
	reports := read("internal/transport/http/reports.go")
	if !strings.Contains(reports, `DefaultContentType: "text/csv; charset=utf-8"`) || !strings.Contains(reports, `DefaultFilename:    "orders.csv"`) || !strings.Contains(reports, "writeFileResponse(w, resp.Body, resp.Content, resp.Size") {
		t.Fatalf("the handler writes the file:\n%s", reports)
	}
	if strings.Contains(reports, `"encoding/json"`) || strings.Contains(reports, "crypto/sha256") {
		t.Fatalf("a group of file endpoints encodes no JSON and has no ETag:\n%s", reports)
	}
	orders := read("internal/transport/http/orders.go")
	if !strings.Contains(orders, `"encoding/json"`) || !strings.Contains(orders, `Disposition:        "inline"`) || strings.Count(orders, "writeFileResponse(") != 1 {
		t.Fatalf("a mixed group keeps JSON for GetOrder and writes the file for InvoicePDF:\n%s", orders)
	}
	if !strings.Contains(read("internal/transport/http/file_response.go"), "func writeFileResponse(") {
		t.Fatal("file_response.go is emitted with the first file endpoint")
	}

	plain := t.TempDir()
	pem := New(plain, filepath.Join(plain, "sdk"), "templates")
	jsonOnly := fileOutputSchema()
	jsonOnly.Services = jsonOnly.Services[1:]
	jsonOnly.Services[0].Methods = jsonOnly.Services[0].Methods[1:]
	jsonOnly.Endpoints = jsonOnly.Endpoints[2:]
	if err := pem.EmitServiceFromIR(jsonOnly); err != nil {
		t.Fatal(err)
	}
	if err := pem.EmitHTTPFromIR(jsonOnly, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(plain, "internal/transport/http/file_response.go")); !os.IsNotExist(err) {
		t.Fatal("no file endpoint, no file_response.go")
	}
}

// The helper itself, compiled and run: headers, the ASCII stand-in and
// filename* for a name that is not ASCII, streaming a Body.
func TestFileResponseHelperWritesHeadersAndBody(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs go test in a temporary module")
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":           "module filehelper\n\ngo 1.22\n",
		"file_response.go": strings.Replace(fileResponseSource, "package http", "package filehelper", 1),
		"helper_test.go": `package filehelper

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHelper(t *testing.T) {
	rec := httptest.NewRecorder()
	writeFileResponse(rec, nil, []byte("a,b\n"), 0, fileResponseHeaders{DefaultContentType: "text/csv", DefaultFilename: "orders.csv", Disposition: "attachment"})
	if got := rec.Header().Get("Content-Disposition"); got != ` + "`" + `attachment; filename="orders.csv"` + "`" + ` {
		t.Fatalf("disposition %q", got)
	}
	if rec.Header().Get("Content-Type") != "text/csv" || rec.Header().Get("Content-Length") != "4" || rec.Body.String() != "a,b\n" {
		t.Fatalf("headers %v body %q", rec.Header(), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	body := io.NopCloser(strings.NewReader("%PDF"))
	writeFileResponse(rec, body, nil, 4, fileResponseHeaders{ContentType: "application/pdf", Filename: "Rechnung März \"1\".pdf", DefaultFilename: "x.pdf", Disposition: "inline"})
	want := ` + "`" + `inline; filename="Rechnung M_rz _1_.pdf"; filename*=UTF-8''Rechnung%20M%C3%A4rz%20%221%22.pdf` + "`" + `
	if got := rec.Header().Get("Content-Disposition"); got != want {
		t.Fatalf("disposition %q, want %q", got, want)
	}
	if rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("Content-Length") != "4" || rec.Body.String() != "%PDF" {
		t.Fatalf("headers %v body %q", rec.Header(), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	writeFileResponse(rec, nil, nil, 0, fileResponseHeaders{Filename: "a\r\nSet-Cookie: x"})
	if got := rec.Header().Get("Content-Disposition"); strings.ContainsAny(got, "\r\n") || got != ` + "`" + `attachment; filename="aSet-Cookie: x"` + "`" + ` {
		t.Fatalf("a name cannot add a header line: %q", got)
	}
	if rec.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated helper: %v\n%s", err, out)
	}
}

func TestOpenAPIDescribesFileOutputsAsBinary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	schema := fileOutputSchema()
	if err := em.EmitOpenAPI(schema.Endpoints, schema.Services, nil, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(data)
	if !strings.Contains(spec, "'text/csv; charset=utf-8':") && !strings.Contains(spec, `"text/csv; charset=utf-8":`) {
		t.Fatalf("the CSV answer is described by its content type:\n%s", spec)
	}
	if !strings.Contains(spec, "format: binary") {
		t.Fatalf("a file is binary:\n%s", spec)
	}
	if strings.Contains(spec, "    ExportOrdersResponse:") {
		t.Fatalf("a file output has no JSON schema:\n%s", spec)
	}
	if !strings.Contains(spec, "    GetOrderResponse:") {
		t.Fatalf("JSON outputs keep their schema:\n%s", spec)
	}
}
