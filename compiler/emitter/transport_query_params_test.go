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

func queryMeta(typ, format string, required bool) map[string]any {
	return map[string]any{normalizer.FieldMetaQuery: map[string]any{"type": typ, "format": format, "required": required}}
}

func queryParamSchema() *ir.Schema {
	str := ir.TypeRef{Kind: ir.KindString}
	return &ir.Schema{
		Services: []ir.Service{{
			Name: "Orders",
			Methods: []ir.Method{
				{
					Name: "ListOrders",
					Input: &ir.Entity{Name: "ListOrdersRequest", Fields: []ir.Field{
						{Name: "status", Type: str},
						{Name: "from", Type: ir.TypeRef{Kind: ir.KindTime}, Metadata: queryMeta("date", "2006-01-02", false)},
						{Name: "page", Type: ir.TypeRef{Kind: ir.KindInt}, Metadata: queryMeta("int", "", true)},
						{Name: "ids", Type: ir.TypeRef{Kind: ir.KindList, ItemType: &str}, Metadata: queryMeta("[]string", "", false)},
					}},
					Output: &ir.Entity{Name: "ListOrdersResponse", Fields: []ir.Field{{Name: "total", Type: ir.TypeRef{Kind: ir.KindInt}}}},
				},
				{
					Name: "CloseOrders",
					Input: &ir.Entity{Name: "CloseOrdersRequest", Fields: []ir.Field{
						{Name: "reason", Type: str},
						{Name: "before", Type: ir.TypeRef{Kind: ir.KindTime}, Metadata: queryMeta("time", "2006-01-02T15:04:05Z07:00", true)},
					}},
					Output: &ir.Entity{Name: "CloseOrdersResponse", Fields: []ir.Field{{Name: "closed", Type: ir.TypeRef{Kind: ir.KindInt}}}},
				},
			},
		}},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/orders", Service: "Orders", RPC: "ListOrders"},
			{Method: "POST", Path: "/orders/close", Service: "Orders", RPC: "CloseOrders"},
		},
	}
}

func TestEmitHTTPParsesTypedQueryParams(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.GoModule = "example.com/shop"
	schema := queryParamSchema()
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitHTTPFromIR(schema, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "internal", "transport", "http", "orders.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, want := range []string{
		`if v := r.URL.Query().Get("status"); v != "" {`, // no @query: unchanged
		`queryParamTime(r, "from", "2006-01-02")`,
		`writeQueryParamError(w, r, "from", "must be a date like 2006-01-02")`,
		`queryParamInt(r, "page")`,
		`writeQueryParamError(w, r, "page", "is required")`,
		`queryParamStrings(r, "ids", "")`,
		`req.IDs = v`,
		`queryParamTime(r, "before", "2006-01-02T15:04:05Z07:00")`, // POST: read after the body
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("handler lacks %q:\n%s", want, src)
		}
	}
	if strings.Contains(src, `"strconv"`) {
		t.Fatalf("typed query parameters parse in the helpers; the handler needs no strconv:\n%s", src)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "transport", "http", "query_params.go")); err != nil {
		t.Fatalf("query_params.go: %v", err)
	}
}

// The generated helpers and handler code, compiled and run against requests.
func TestTypedQueryParamsAnswerOneProblemShape(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs go test in a temporary module")
	}
	field := func(name, goType string, meta map[string]any) normalizer.Field {
		return normalizer.Field{Name: name, Type: goType, Metadata: meta}
	}
	code := strings.Join([]string{
		renderQueryParamCode(field("from", "time.Time", queryMeta("date", "2006-01-02", false))),
		renderQueryParamCode(field("page", "int", queryMeta("int", "", true))),
		renderQueryParamCode(field("ids", "[]string", queryMeta("[]string", "", false))),
		renderQueryParamCode(field("tags", "[]string", queryMeta("[]string", "csv", false))),
		renderQueryParamCode(field("debug", "bool", queryMeta("bool", "", false))),
	}, "\n")
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/shop\n\ngo 1.22\n",
		"internal/pkg/errors/errors.go": `package errors

import (
	"encoding/json"
	"net/http"
)

type Problem struct {
	Title  string            ` + "`json:\"title\"`" + `
	Errors map[string]string ` + "`json:\"errors\"`" + `
}

func NewValidationError(title string, fields map[string]string) *Problem {
	return &Problem{Title: title, Errors: fields}
}

func WriteError(w http.ResponseWriter, r *http.Request, p *Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(p)
}
`,
		"internal/transport/http/query_params.go": queryParamsSource("example.com/shop"),
		"internal/transport/http/handler_test.go": `package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type listRequest struct {
	From  time.Time
	Page  int
	IDs   []string
	Tags  []string
	Debug bool
}

func handle(w http.ResponseWriter, r *http.Request) {
	var req listRequest
` + code + `
	fmt.Fprintf(w, "%s|%d|%s|%s|%v", req.From.Format("2006-01-02"), req.Page, strings.Join(req.IDs, ","), strings.Join(req.Tags, ","), req.Debug)
}

func call(query string) (int, string) {
	rec := httptest.NewRecorder()
	handle(rec, httptest.NewRequest("GET", "/orders?"+query, nil))
	return rec.Code, strings.TrimSpace(rec.Body.String())
}

func TestQuery(t *testing.T) {
	cases := []struct{ query string; code int; body string }{
		{"page=2&from=2026-10-14&ids=a&ids=b,c&tags=x,y&tags=z&debug=true", 200, "2026-10-14|2|a,b,c|x,y|true"},
		{"page=1", 200, "0001-01-01|1|||false"},
		{"page=1&from=14.10.2026", 400, ` + "`" + `{"title":"Invalid query","errors":{"from":"must be a date like 2006-01-02"}}` + "`" + `},
		{"from=2026-10-14", 400, ` + "`" + `{"title":"Invalid query","errors":{"page":"is required"}}` + "`" + `},
		{"page=two", 400, ` + "`" + `{"title":"Invalid query","errors":{"page":"must be an integer"}}` + "`" + `},
		{"page=1&debug=maybe", 400, ` + "`" + `{"title":"Invalid query","errors":{"debug":"must be true or false"}}` + "`" + `},
	}
	for _, c := range cases {
		code, body := call(c.query)
		if code != c.code || body != c.body {
			t.Errorf("?%s: %d %s, want %d %s", c.query, code, body, c.code, c.body)
		}
	}
}
`,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated query parameter code: %v\n%s", err, out)
	}
}

func TestOpenAPIDescribesTypedQueryParams(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	schema := queryParamSchema()
	if err := em.EmitOpenAPI(schema.Endpoints, schema.Services, nil, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(data)
	for _, want := range []string{
		"        - in: query\n          name: from\n          schema:\n            type: string\n            format: date",
		"        - in: query\n          name: page\n          required: true\n          schema:\n            type: integer",
		"        - in: query\n          name: ids\n          style: form\n          explode: true\n          schema:\n            type: array\n            items:\n              type: string",
		"      parameters:\n        - in: query\n          name: before\n          required: true\n          schema:\n            type: string\n            format: date-time\n      requestBody:",
	} {
		if !strings.Contains(spec, want) {
			t.Fatalf("openapi lacks\n%s\n---\n%s", want, spec)
		}
	}
}
