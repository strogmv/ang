package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

func runtimeTestContext() MainContext {
	return MainContext{
		ServicesIR: []ir.Service{{
			Name:       "Billing",
			Subscribes: map[string]string{"SweepRequested": "Sweep", "InvoicePaid": "Notify"},
			Methods: []ir.Method{
				{Name: "Sweep", Metadata: map[string]any{normalizer.MetaSubscribeWhen: "cfg.JobsEnabled"}},
				{Name: "Notify"},
			},
		}},
		HasNats:           true,
		HasSQL:            true,
		HasScheduler:      true,
		WebSocketServices: map[string]bool{},
		WSEventMap:        map[string]map[string]bool{},
		EventPayloadsIR:   map[string]ir.Entity{},
		GoModule:          "github.com/acme/demo",
		ANGVersion:        "test",
		InputHash:         "test",
		CompilerHash:      "test",
	}
}

func emitTestMain(t *testing.T, ctx MainContext, config *ir.Config) (string, error) {
	t.Helper()
	dir := t.TempDir()
	em := New(dir, "sdk", "templates")
	em.GoModule = ctx.GoModule
	if config != nil {
		em.IRSchema = &ir.Schema{Config: *config}
	}
	if err := em.EmitMain(ctx); err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "cmd", "server", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data), nil
}

func jobsConfig() *ir.Config {
	return &ir.Config{Fields: []ir.Field{{Name: "JobsEnabled"}}}
}

// Item 24 of docs/tech-debt-backlog.md: target.server_roles makes one binary
// an API (routes), a worker (subscriptions, schedules) or both.
func TestMainServerRoles(t *testing.T) {
	ctx := runtimeTestContext()
	ctx.ServerRoles = true
	out, err := emitTestMain(t, ctx, jobsConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`serverRole := os.Getenv("SERVER_ROLE")`,
		`serverRole != "all" && serverRole != "api" && serverRole != "worker"`,
		"serveAPI := serverRole != \"worker\"",
		"runWorkers := serverRole != \"api\"",
		"if natsClient != nil && runWorkers {",
		"if runWorkers {\n\t\tsched := scheduler.New(publisher, scheduler.DefaultSchedules)\n\t\tsched.Start(runCtx)",
		"if serveAPI {\n\t\ttransport.RegisterBillingRoutes(r, runtimeContainer.SvcBilling)\n\t}",
		`shutdownTimeout, _ := time.ParseDuration("30s")`,
		`os.Getenv("SHUTDOWN_TIMEOUT")`,
		"stopRun()",
		`r.Get("/health", transport.HealthHandler())`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("main.go misses %q", want)
		}
	}
	if t.Failed() {
		t.Log(out)
	}
}

// subscribe: {when: "cfg.JobsEnabled"} keeps a replica with the jobs off out
// of the queue group; other subscriptions of the service are not affected.
func TestMainServerSubscribeWhen(t *testing.T) {
	out, err := emitTestMain(t, runtimeTestContext(), jobsConfig())
	if err != nil {
		t.Fatal(err)
	}
	sweep := strings.Index(out, `SubscribeQueueRaw("SweepRequested"`)
	notify := strings.Index(out, `SubscribeQueueRaw("InvoicePaid"`)
	guard := strings.Index(out, "if cfg.JobsEnabled {")
	if sweep < 0 || notify < 0 || guard < 0 || !(guard > notify && guard < sweep) {
		t.Fatalf("SweepRequested must be guarded by cfg.JobsEnabled and InvoicePaid not (guard %d, notify %d, sweep %d):\n%s", guard, notify, sweep, out)
	}
	if strings.Count(out, "if cfg.JobsEnabled {") != 1 {
		t.Fatalf("one guard expected:\n%s", out)
	}
	// Without roles, hooks or queues the main keeps today's shape.
	for _, unwanted := range []string{"serverRole", "runCtx", "runWorkers", "shutdownTimeout"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("main.go without runtime options has %q", unwanted)
		}
	}
	if !strings.Contains(out, "sched.Start(ctx)") || !strings.Contains(out, "5*time.Second") {
		t.Errorf("main.go without runtime options changed its scheduler start or shutdown:\n%s", out)
	}
}

func TestMainServerRuntimeRejectsUnknownConfigField(t *testing.T) {
	ctx := runtimeTestContext()
	ctx.ServicesIR[0].Methods[0].Metadata[normalizer.MetaSubscribeWhen] = "cfg.JobsEnabeld && cfg.PGMaxConns > 0"
	ctx.ServicesIR[0].Methods[0].Source = "cue/api/billing.cue:12"
	_, err := emitTestMain(t, ctx, jobsConfig())
	if err == nil || !strings.Contains(err.Error(), "cue/api/billing.cue:12: Billing.Sweep") || !strings.Contains(err.Error(), "cfg.JobsEnabeld, which is not a config field") {
		t.Fatalf("err = %v", err)
	}
	ctx = runtimeTestContext()
	ctx.ShutdownTimeout = "soon"
	if _, err := emitTestMain(t, ctx, jobsConfig()); err == nil || !strings.Contains(err.Error(), "shutdown_timeout") {
		t.Fatalf("err = %v", err)
	}
}

// Item 25: OnStart hooks run before the server serves (failure exits),
// OnShutdown hooks after the drain, in reverse service order, under their
// role and when: condition.
func TestMainServerLifecycleHooks(t *testing.T) {
	ctx := runtimeTestContext()
	ctx.ServicesIR = append(ctx.ServicesIR, ir.Service{Name: "Catalog", Methods: []ir.Method{
		{Name: "WarmCatalog", Metadata: map[string]any{normalizer.MetaHookOn: "OnStart", normalizer.MetaHookRole: "all"}},
		{Name: "FlushCatalog", Metadata: map[string]any{normalizer.MetaHookOn: "OnShutdown", normalizer.MetaHookRole: "worker", normalizer.MetaHookWhen: "cfg.JobsEnabled"}},
	}})
	ctx.ServicesIR[0].Methods = append(ctx.ServicesIR[0].Methods, ir.Method{Name: "FlushBilling", Metadata: map[string]any{normalizer.MetaHookOn: "OnShutdown", normalizer.MetaHookRole: "api"}})
	ctx.ShutdownTimeout = "45s"
	out, err := emitTestMain(t, ctx, jobsConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"serveAPI, runWorkers := true, true",
		`shutdownTimeout, _ := time.ParseDuration("45s")`,
		"if _, err := runtimeContainer.SvcCatalog.WarmCatalog(runCtx, port.WarmCatalogRequest{}); err != nil {\n\t\tlog.Error(\"OnStart hook failed\", \"hook\", \"Catalog.WarmCatalog\", \"error\", err)\n\t\tos.Exit(1)",
		"if runWorkers && (cfg.JobsEnabled) {\n\t\tif _, err := runtimeContainer.SvcCatalog.FlushCatalog(hookCtx, port.FlushCatalogRequest{}); err != nil {",
		"if serveAPI {\n\t\tif _, err := runtimeContainer.SvcBilling.FlushBilling(hookCtx, port.FlushBillingRequest{}); err != nil {",
		"shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("main.go misses %q", want)
		}
	}
	start, serve := strings.Index(out, "WarmCatalog(runCtx"), strings.Index(out, "srv.ListenAndServe()")
	if start < 0 || serve < 0 || start > serve {
		t.Errorf("OnStart hook must run before the server listens")
	}
	stop, catalog, billing := strings.Index(out, "stopRun()\n"), strings.Index(out, "FlushCatalog(hookCtx"), strings.Index(out, "FlushBilling(hookCtx")
	if !(stop > 0 && stop < catalog && catalog < billing) {
		t.Errorf("OnShutdown hooks run after stopRun, Catalog (after Billing in service order) first: stop %d catalog %d billing %d", stop, catalog, billing)
	}
	if t.Failed() {
		t.Log(out)
	}
}
