package emitter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

func operationMetricsSchema() *ir.Schema {
	return &ir.Schema{Services: []ir.Service{{
		Name: "Orders",
		Methods: []ir.Method{
			{
				Name:   "GetOrder",
				Input:  &ir.Entity{Name: "GetOrderRequest", Fields: []ir.Field{{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}}},
				Output: &ir.Entity{Name: "GetOrderResponse", Fields: []ir.Field{{Name: "total", Type: ir.TypeRef{Kind: ir.KindInt}}}},
				Impl:   &ir.Impl{Lang: "go", Code: "resp.Total = 1\nreturn resp, nil"},
			},
			{
				Name:        "StreamOrders",
				IsStreaming: true,
				Input:       &ir.Entity{Name: "StreamOrdersRequest"},
				Impl:        &ir.Impl{Lang: "go", Code: "return nil"},
			},
		},
	}}}
}

func emitOperationMetricsProject(t *testing.T, on bool) string {
	t.Helper()
	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.GoModule = "example.com/shop"
	em.OperationMetrics = on
	schema := operationMetricsSchema()
	em.IRSchema = schema
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitServiceImpl(schema.Services, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestOperationMetricsWrapEveryOperation(t *testing.T) {
	t.Parallel()

	root := emitOperationMetricsProject(t, true)
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	get := read("internal/service/orders__get_order.gen.go")
	if !strings.Contains(get, `ctx, observeDone := observeOperation(ctx, "Orders", "GetOrder")`) || !strings.Contains(get, "defer func() { observeDone(err) }()") {
		t.Fatalf("GetOrder runs inside observeOperation:\n%s", get)
	}
	if stream := read("internal/service/orders__stream_orders.gen.go"); strings.Contains(stream, "observeOperation") {
		t.Fatalf("a streaming operation has no named error to observe:\n%s", stream)
	}
	if helper := read("internal/service/" + operationMetricsFile); !strings.Contains(helper, "ang_operation_duration_seconds") {
		t.Fatal("the operation timer is generated")
	}
	if tracer := read("internal/pkg/dbobserve/dbobserve.go"); !strings.Contains(tracer, "func (QueryTracer) TraceQueryEnd(") {
		t.Fatal("the query tracer is generated")
	}

	off := emitOperationMetricsProject(t, false)
	data, err := os.ReadFile(filepath.Join(off, "internal/service/orders__get_order.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "observeOperation") {
		t.Fatal("without operation_metrics the operation is generated as before")
	}
	for _, rel := range []string{"internal/service/" + operationMetricsFile, "internal/pkg/dbobserve/dbobserve.go"} {
		if _, err := os.Stat(filepath.Join(off, rel)); !os.IsNotExist(err) {
			t.Fatalf("without operation_metrics there is no %s", rel)
		}
	}
}

func TestMainServerTracesQueriesUnderOperationMetrics(t *testing.T) {
	t.Parallel()

	for _, on := range []bool{true, false} {
		em := New(t.TempDir(), "", "templates")
		em.OperationMetrics = on
		tmpl, err := em.parseMainServerTemplate()
		if err != nil {
			t.Fatal(err)
		}
		infra := tmpl.Lookup("main_server_infrastructure")
		imports := tmpl.Lookup("main_server_imports")
		if infra == nil || imports == nil {
			t.Fatal("main server templates missing")
		}
		var b strings.Builder
		ctx := MainContext{HasSQL: true, GoModule: "example.com/shop"}
		if err := infra.Execute(&b, ctx); err != nil {
			t.Fatal(err)
		}
		if err := imports.Execute(&b, ctx); err != nil {
			t.Fatal(err)
		}
		src := b.String()
		has := strings.Contains(src, "poolCfg.ConnConfig.Tracer = dbobserve.QueryTracer{}") && strings.Contains(src, `"example.com/shop/internal/pkg/dbobserve"`)
		if has != on {
			t.Fatalf("operation_metrics=%v: tracer wired=%v\n%s", on, has, src)
		}
	}
}

// The generated tracer, compiled against pgx, Prometheus and OpenTelemetry
// from the module cache and run: a query started in a repository method is
// recorded under that method, with its rows.
func TestQueryTracerRecordsTheRepositoryMethod(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds pgx, Prometheus and OpenTelemetry in a temporary module")
	}
	// ANG's own requirements: the versions its build already put in the
	// module cache, so the temporary module resolves offline.
	angMod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Skipf("ANG go.mod: %v", err)
	}
	gomod := strings.Replace(string(angMod), "module github.com/strogmv/ang", "module example.com/shop", 1)
	for _, dep := range []string{"github.com/jackc/pgx/v5", "github.com/prometheus/client_golang", "go.opentelemetry.io/otel ", "go.opentelemetry.io/otel/trace"} {
		if !strings.Contains(gomod, dep) {
			t.Skipf("ANG no longer requires %s", dep)
		}
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":                              gomod,
		"internal/pkg/dbobserve/dbobserve.go": dbObserveSource,
		"internal/adapter/repository/postgres/repo_test.go": `package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"example.com/shop/internal/pkg/dbobserve"
)

type TenderRepository struct{}

func (r *TenderRepository) ListByCompany(ctx context.Context, fail bool) {
	tracer := dbobserve.QueryTracer{}
	ctx = tracer.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	if fail {
		tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("boom")})
		return
	}
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag("SELECT 7")})
}

func metric(t *testing.T, name string) []*dto.Metric {
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == name {
			return f.GetMetric()
		}
	}
	return nil
}

func labels(m *dto.Metric) map[string]string {
	out := map[string]string{}
	for _, l := range m.GetLabel() {
		out[l.GetName()] = l.GetValue()
	}
	return out
}

func TestTracer(t *testing.T) {
	repo := &TenderRepository{}
	repo.ListByCompany(context.Background(), false)
	repo.ListByCompany(context.Background(), true)

	rows := metric(t, "ang_db_query_rows")
	if len(rows) != 1 || labels(rows[0])["repository"] != "TenderRepository" || labels(rows[0])["method"] != "ListByCompany" || rows[0].GetHistogram().GetSampleSum() != 7 {
		t.Fatalf("rows %v", rows)
	}
	errs := metric(t, "ang_db_query_errors_total")
	if len(errs) != 1 || errs[0].GetCounter().GetValue() != 1 {
		t.Fatalf("errors %v", errs)
	}
	outcomes := map[string]uint64{}
	for _, m := range metric(t, "ang_db_query_duration_seconds") {
		outcomes[labels(m)["outcome"]] = m.GetHistogram().GetSampleCount()
	}
	if outcomes["ok"] != 1 || outcomes["error"] != 1 {
		t.Fatalf("durations %v", outcomes)
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
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
	out, err := cmd.CombinedOutput()
	if err != nil && strings.Contains(string(out), "GOPROXY=off") {
		t.Skipf("modules not in the local cache: %s", out)
	}
	if err != nil {
		t.Fatalf("generated query tracer: %v\n%s", err, out)
	}
}
