package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// requestLogMaskTest runs inside the generated transport package: requests
// carrying credentials in the path, the query and a header go through the
// access log, the error log and the metrics, and none of them may keep one.
const requestLogMaskTest = `package http

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/example/maskapp/internal/pkg/errors"
)

func TestRequestLogAndMetricsCarryNoCredentials(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	r := chi.NewRouter()
	r.Use(MetricsMiddleware, LoggingMiddleware)
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	r.Get("/api/public/contracts/sign/{token}/pdf", ok)
	r.Post("/api/public/telegram/{botId}/{secretToken:[A-Za-z0-9_-]+}", func(w http.ResponseWriter, r *http.Request) {
		errors.WriteError(w, r, errors.New(http.StatusInternalServerError, "Internal Server Error", "webhook failed"))
	})
	r.Get("/api/items/{id}", ok)
	r.Route("/api/carts", func(sub chi.Router) {
		sub.Get("/{cartToken}/items", ok)
	})

	const (
		signToken  = "sign-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		hookSecret = "hook-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
		cartToken  = "cart-CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
		sessionID  = "sess-DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"
		lostToken  = "EEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE"
	)
	send := func(method, target string) {
		req := httptest.NewRequest(method, target, nil)
		req.Header.Set("X-Auth-Session", sessionID)
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", hookSecret)
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	send(http.MethodGet, "/api/public/contracts/sign/"+signToken+"/pdf")
	send(http.MethodPost, "/api/public/telegram/bot7/"+hookSecret)
	send(http.MethodGet, "/api/items/42?session="+sessionID+"&page=2")
	send(http.MethodGet, "/api/carts/"+cartToken+"/items")
	send(http.MethodGet, "/api/nowhere/"+lostToken)

	logs := buf.String()
	for _, secret := range []string{signToken, hookSecret, cartToken, sessionID, lostToken} {
		if strings.Contains(logs, secret) {
			t.Errorf("a log line carries %s:\n%s", secret, logs)
		}
	}
	for _, want := range []string{
		` + "`" + `"path":"/api/public/contracts/sign/***/pdf"` + "`" + `,
		` + "`" + `"route":"/api/public/contracts/sign/{token}/pdf"` + "`" + `,
		` + "`" + `"path":"/api/public/telegram/bot7/***"` + "`" + `,
		` + "`" + `"path":"/api/items/42"` + "`" + `,
		` + "`" + `"path":"/api/carts/***/items"` + "`" + `,
		` + "`" + `"path":"/api/nowhere/***"` + "`" + `,
		"page=2",
		"session=%2A%2A%2A",
		"Application error response",
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %s:\n%s", want, logs)
		}
	}

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]bool{}
	for _, mf := range families {
		if !strings.HasPrefix(mf.GetName(), "http_") {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "path" {
					labels[l.GetValue()] = true
				}
			}
		}
	}
	for label := range labels {
		for _, secret := range []string{signToken, hookSecret, cartToken, lostToken, "/api/items/42"} {
			if strings.Contains(label, secret) {
				t.Errorf("metric label %q carries %s", label, secret)
			}
		}
	}
	for _, want := range []string{"/api/carts/{cartToken}/items", "/api/items/{id}", "unmatched"} {
		if !labels[want] {
			t.Errorf("metric labels lack %q: %v", want, labels)
		}
	}
}
`

// The request log wrote r.URL.Path (share-link, contract-signing and cart
// tokens, the Telegram webhook secret), the error log of WriteError did too,
// and http_requests_inflight was labelled with the raw path (dealingi-back
// audit item 25; backlog 31). Now each line carries the path with every
// sensitive parameter masked plus the route pattern, the query with its
// sensitive values masked, no header, and metrics carry route patterns only.
func TestRequestLogAndMetricsMaskCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and tests a generated project")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	projectDir := filepath.Join(t.TempDir(), "maskapp")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "maskapp",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/maskapp",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := runBuild([]string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}); err != nil {
		t.Fatalf("build: %v", err)
	}
	writeTestFile(t, filepath.Join(projectDir, "internal", "transport", "http", "request_log_mask_test.go"), requestLogMaskTest)
	runGoOffline(t, projectDir, "github.com/example/maskapp", "test", "-count=1", "-run", "TestRequestLogAndMetricsCarryNoCredentials", "./internal/transport/http/")
}
