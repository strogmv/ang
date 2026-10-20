package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const metricsHandlerTest = `package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsHandlerAsksForTheToken(t *testing.T) {
	get := func(h http.Handler, auth string) int {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	guarded := MetricsHandler("s3cret")
	if code := get(guarded, ""); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", code)
	}
	if code := get(guarded, "Bearer wrong"); code != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", code)
	}
	if code := get(guarded, "Bearer s3cret"); code != http.StatusOK {
		t.Fatalf("right token: %d", code)
	}
	if code := get(MetricsHandler(""), ""); code != http.StatusOK {
		t.Fatalf("no token configured: %d", code)
	}
}
`

// /metrics was mounted on the API router with no authentication, so anyone
// on the internet read route names, error rates and process internals
// (dealingi-back audit). target.metrics_port (METRICS_PORT at run time)
// serves it on a port of its own and takes it off the API port; without one
// it stays on the API port behind METRICS_TOKEN when that is set.
func TestMetricsLeaveTheAPIPort(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and tests a generated project")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	projectDir := filepath.Join(t.TempDir(), "metricsapp")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "metricsapp",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/metricsapp",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	projectCUE := filepath.Join(projectDir, "cue", "project", "project.cue")
	raw, err := os.ReadFile(projectCUE)
	if err != nil {
		t.Fatal(err)
	}
	withPort := strings.Replace(string(raw), "\t\toutput_dir:", "\t\tmetrics_port: \"9090\"\n\t\toutput_dir:", 1)
	if withPort == string(raw) {
		t.Fatalf("scaffold project.cue changed shape:\n%s", raw)
	}
	writeTestFile(t, projectCUE, withPort)
	if err := runBuild([]string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}); err != nil {
		t.Fatalf("build: %v", err)
	}
	main, err := os.ReadFile(filepath.Join(projectDir, "cmd", "server", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`metricsPort = "9090"`, `os.Getenv("METRICS_PORT")`, `os.Getenv("METRICS_TOKEN")`, `metricsMux.Handle("/metrics", transport.MetricsHandler(metricsToken))`} {
		if !strings.Contains(string(main), want) {
			t.Errorf("main.go lacks %s", want)
		}
	}
	if strings.Contains(string(main), "promhttp.Handler()") {
		t.Error("main.go still mounts the open handler")
	}
	writeTestFile(t, filepath.Join(projectDir, "internal", "transport", "http", "metrics_handler_test.go"), metricsHandlerTest)
	// cmd/server needs the OTLP exporter, which the offline module cache may
	// lack; dealingi-back's build compiles the main with this template.
	runGoOffline(t, projectDir, "github.com/example/metricsapp", "test", "-count=1", "-run", "TestMetricsHandlerAsksForTheToken", "./internal/transport/http/")
	k8s, err := os.ReadFile(filepath.Join(projectDir, "deploy", "k8s", "auth-deployment.yaml"))
	if err == nil && (!strings.Contains(string(k8s), "containerPort: 9090") || strings.Contains(string(k8s), "path: /metrics")) {
		t.Errorf("k8s deployment: metrics port listed, liveness on /health:\n%s", k8s)
	}
}
