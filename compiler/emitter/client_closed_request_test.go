package emitter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// A request whose client went away is not a server error. The generated
// errors package answers it with 499 (nothing is sent to anyone) and logs it
// quietly; a server-side deadline and a database without free connections are
// 503 with Retry-After; anything else stays a 500. The generated package is
// compiled and its behaviour exercised, not just grepped.
func TestEmitErrors_ClientClosedRequestAndBusyDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a generated package")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not found")
	}
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	if err := em.EmitErrors(nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/gen\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const behaviour = `package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pgErr struct{ code string }

func (e *pgErr) Error() string    { return "FATAL: sorry, too many clients already (SQLSTATE " + e.code + ")" }
func (e *pgErr) SQLState() string { return e.code }

func request(ctx context.Context) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/api/x", nil).WithContext(ctx)
}

func TestClientGoneIs499(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := request(ctx)
	for _, err := range []error{context.Canceled, fmt.Errorf("FindByID scan: %w", context.Canceled), New(http.StatusInternalServerError, "Internal Server Error", "verification check failed"), New(http.StatusServiceUnavailable, "SESSION_STORE_UNAVAILABLE", "x")} {
		w := httptest.NewRecorder()
		WriteError(w, r, err)
		if w.Code != StatusClientClosedRequest || w.Body.Len() != 0 {
			t.Fatalf("%v: got %d %q, want 499 without a body", err, w.Code, w.Body.String())
		}
	}
	if !ClientGone(r) || ObservedStatus(r, 503) != 499 || ObservedStatus(r, 0) != 499 || ObservedStatus(r, 200) != 200 {
		t.Fatal("ObservedStatus does not map a cancelled request to 499")
	}
}

func TestDeadlineAndBusyDatabaseAre503WithRetryAfter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	<-ctx.Done()
	for name, tc := range map[string]struct {
		r   *http.Request
		err error
	}{
		"deadline":            {request(ctx), fmt.Errorf("acquire: %w", context.DeadlineExceeded)},
		"too many clients":    {request(context.Background()), fmt.Errorf("query: %w", &pgErr{"53300"})},
		"too many (text only)": {request(context.Background()), stderrors.New("failed to connect: FATAL: sorry, too many clients already (SQLSTATE 53300)")},
		"starting up":         {request(context.Background()), &pgErr{"57P03"}},
	} {
		w := httptest.NewRecorder()
		WriteError(w, tc.r, tc.err)
		if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
			t.Fatalf("%s: got %d Retry-After=%q, want 503 with Retry-After", name, w.Code, w.Header().Get("Retry-After"))
		}
		if ClientGone(tc.r) || ObservedStatus(tc.r, 503) != 503 {
			t.Fatalf("%s: a server-side timeout is not a client that left", name)
		}
	}
}

func TestOtherErrorsStay500(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, request(context.Background()), stderrors.New("boom"))
	if w.Code != http.StatusInternalServerError || w.Header().Get("Retry-After") != "" {
		t.Fatalf("got %d Retry-After=%q, want a plain 500", w.Code, w.Header().Get("Retry-After"))
	}
}
`
	if err := os.WriteFile(filepath.Join(tmp, "internal", "pkg", "errors", "behaviour_test.go"), []byte(behaviour), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goBin, "test", "./internal/pkg/errors/")
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated errors package: %v\n%s", err, out)
	}
}

// Access logs, metrics and the route timeout record a client that left as
// 499; the session check does not report the store as unavailable then.
func TestEmitHTTPCommon_ClientClosedRequestIsNot5xx(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	if err := em.EmitHTTPCommon(&normalizer.AuthDef{Mode: "opaque_session_cookie"}); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitLoggingMiddleware(); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitMetrics(); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	common := read("common.go")
	for _, want := range []string{
		"th.ServeHTTP(&timeoutStatusWriter{ResponseWriter: w, r: r}, r)",
		"code = errors.StatusClientClosedRequest",
		`tw.Header().Set("Retry-After", errors.RetryAfterSeconds)`,
	} {
		if !strings.Contains(common, want) {
			t.Fatalf("generated common.go is missing %q", want)
		}
	}
	store := common[strings.Index(common, "case *sessionStoreError:"):]
	if strings.Index(store, "errors.ClientGone(r)") > strings.Index(store, `slog.Warn("auth: session store unavailable"`) {
		t.Fatal("a cancelled request is still reported as SESSION_STORE_UNAVAILABLE")
	}
	if !strings.Contains(read("logging.go"), "status := errors.ObservedStatus(r, ww.Status())") {
		t.Fatal("the access log does not record 499 for a client that left")
	}
	if !strings.Contains(read("metrics.go"), "errors.ObservedStatus(r, ww.Status())") {
		t.Fatal("metrics do not record 499 for a client that left")
	}
}
