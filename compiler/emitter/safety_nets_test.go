package emitter

import (
	"strings"
	"testing"
)

func readTemplate(t *testing.T, path string) string {
	t.Helper()
	raw, err := ReadTemplateByPath(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// section returns the text of a Go function in a template, from its
// signature to the next top-level func.
func section(t *testing.T, src, signature string) string {
	t.Helper()
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("%q not found", signature)
	}
	rest := src[start+len(signature):]
	if end := strings.Index(rest, "\nfunc "); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

// The hub closes a client's send queue when it evicts the client; readPump
// runs concurrently and must never send on that queue itself (send on a
// closed channel panicked the whole process).
func TestWSReadPumpNeverSendsOnClientQueue(t *testing.T) {
	src := readTemplate(t, "templates/websocket_common.tmpl")
	body := section(t, src, "func (c *wsClient) readPump() {")
	if strings.Contains(body, "c.send <-") {
		t.Fatalf("readPump writes to c.send directly; route replies through the hub (sendDirect)")
	}
	if !strings.Contains(body, "recover()") {
		t.Fatalf("readPump must recover panics")
	}
	if !strings.Contains(section(t, src, "func (c *wsClient) writePump() {"), "recover()") {
		t.Fatalf("writePump must recover panics")
	}
}

// A panic while the hub holds h.mu must not leave the mutex locked.
func TestWSHubUnlocksThroughDefer(t *testing.T) {
	src := readTemplate(t, "templates/websocket_common.tmpl")
	body := section(t, src, "func (h *wsHub) handleOne() bool {")
	locks := strings.Count(body, "h.mu.Lock()")
	deferred := strings.Count(body, "defer h.mu.Unlock()")
	if locks == 0 || locks != deferred {
		t.Fatalf("hub branches: %d locks, %d deferred unlocks", locks, deferred)
	}
}

// WithTx called with a ctx that already carries a transaction joins it as a
// savepoint instead of opening an independent transaction.
func TestWithTxJoinsOuterTransaction(t *testing.T) {
	src := readTemplate(t, "templates/postgres_common.tmpl")
	body := section(t, src, "func (tm *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {")
	join := strings.Index(body, "ctx.Value(txKey{}).(pgx.Tx)")
	begin := strings.Index(body, "tm.pool.Begin(ctx)")
	if join < 0 || begin < 0 || join > begin || !strings.Contains(body, "outer.Begin(ctx)") {
		t.Fatalf("WithTx must join a transaction already in ctx (savepoint) before beginning a new one")
	}
}

// Background goroutines of the generated server recover panics.
func TestBackgroundGoroutinesRecover(t *testing.T) {
	cases := map[string][]string{
		"templates/scheduler.tmpl":               {"func safePublish(", "recover()"},
		"templates/http.tmpl":                    {"errCh <- logger.PanicError(\"sse\""},
		"templates/websocket_live.tmpl":          {"logger.Recovered(\"live\", ep.name+\" refresh\"", "logger.Recovered(\"live\", ep.name+\" trigger \""},
		"templates/nats_client_v2.tmpl":          {"logger.HandlerFailed(\"nats\", subject, handler(msg.Data))", "logger.Go(\"nats\", name"},
		"templates/main_server/services.tmpl":    {"handlerErr = logger.PanicError(\"nats\""},
		"templates/main_server/websockets.tmpl":  {"handlerErr = logger.PanicError(\"nats-ws\""},
		"templates/main_server/http_router.tmpl": {"r.Use(transport.RecoverMiddleware)"},
		"templates/metrics.tmpl":                 {"func RecoverMiddleware(", "ang_panics_recovered_total", "ang_event_handler_errors_total"},
	}
	for path, wants := range cases {
		src := readTemplate(t, path)
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Errorf("%s: missing %q", path, want)
			}
		}
	}
	if strings.Contains(readTemplate(t, "templates/main_server/http_router.tmpl"), "middleware.Recoverer") {
		t.Errorf("http_router.tmpl still uses chi's Recoverer (no metric, stderr only)")
	}
}
