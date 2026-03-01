package main

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDetectComposeCommandPrefersDockerPlugin(t *testing.T) {
	tmp := t.TempDir()
	writeExecutable(t, filepath.Join(tmp, "docker"), "#!/bin/sh\nif [ \"$1\" = \"compose\" ] && [ \"$2\" = \"version\" ]; then exit 0; fi\nexit 1\n")
	writeExecutable(t, filepath.Join(tmp, "docker-compose"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", tmp)

	cmd, err := detectComposeCommand()
	if err != nil {
		t.Fatalf("detectComposeCommand: %v", err)
	}
	if len(cmd) != 2 || cmd[0] != "docker" || cmd[1] != "compose" {
		t.Fatalf("expected docker compose, got %#v", cmd)
	}
}

func TestDetectComposeCommandFallbackToDockerComposeBinary(t *testing.T) {
	tmp := t.TempDir()
	writeExecutable(t, filepath.Join(tmp, "docker"), "#!/bin/sh\nexit 1\n")
	composePath := filepath.Join(tmp, "docker-compose")
	writeExecutable(t, composePath, "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", tmp)

	cmd, err := detectComposeCommand()
	if err != nil {
		t.Fatalf("detectComposeCommand: %v", err)
	}
	if len(cmd) != 1 || cmd[0] != composePath {
		t.Fatalf("expected docker-compose fallback, got %#v", cmd)
	}
}

func TestDetectComposeCommandUnavailable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)

	_, err := detectComposeCommand()
	if err == nil {
		t.Fatalf("expected error when compose is unavailable")
	}
}

func TestCollectConfigStartupChecksMissingRequiredConfig(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "internal", "config", "config.go")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package config

type Config struct {
	DatabaseURL string ` + "`env:\"DATABASE_URL\" env-required:\"true\"`" + `
}
`
	if err := os.WriteFile(cfgPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env.example"), []byte("DATABASE_URL=\n"), 0o644); err != nil {
		t.Fatalf("write .env.example: %v", err)
	}

	checks, err := collectConfigStartupChecks(root)
	if err != nil {
		t.Fatalf("collectConfigStartupChecks: %v", err)
	}
	cfg := startupCheckByName(checks, "config-env")
	if cfg == nil {
		t.Fatalf("missing config-env check in %#v", checks)
	}
	if cfg.Status != startupFail {
		t.Fatalf("expected config-env fail, got %s (%s)", cfg.Status, cfg.Detail)
	}
	if !strings.Contains(cfg.Detail, "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL in detail, got %q", cfg.Detail)
	}
}

func TestCollectConfigStartupChecksAutoBootstrapsEnv(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "internal", "config", "config.go")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package config

type Config struct {
	DatabaseURL string ` + "`env:\"DATABASE_URL\" env-required:\"true\"`" + `
}
`
	if err := os.WriteFile(cfgPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env.example"), []byte("DATABASE_URL=postgres://app:app@localhost:5432/app?sslmode=disable\nJWT_PRIVATE_KEY=secret-key\n"), 0o644); err != nil {
		t.Fatalf("write .env.example: %v", err)
	}

	checks, err := collectConfigStartupChecks(root)
	if err != nil {
		t.Fatalf("collectConfigStartupChecks: %v", err)
	}

	bootstrap := startupCheckByName(checks, ".env.bootstrap")
	if bootstrap == nil {
		t.Fatalf("expected .env.bootstrap check in %#v", checks)
	}
	if bootstrap.Status != startupOK {
		t.Fatalf("expected .env.bootstrap ok, got %s (%s)", bootstrap.Status, bootstrap.Detail)
	}

	cfg := startupCheckByName(checks, "config-env")
	if cfg == nil {
		t.Fatalf("missing config-env check in %#v", checks)
	}
	if cfg.Status != startupOK {
		t.Fatalf("expected config-env ok, got %s (%s)", cfg.Status, cfg.Detail)
	}

	envData, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if !strings.Contains(string(envData), "DATABASE_URL=postgres://app:app@localhost:5432/app?sslmode=disable") {
		t.Fatalf(".env missing DATABASE_URL autofill:\n%s", string(envData))
	}
}

func TestCollectConfigStartupChecksWarnWhenSchemaMissing(t *testing.T) {
	root := t.TempDir()

	checks, err := collectConfigStartupChecks(root)
	if err != nil {
		t.Fatalf("collectConfigStartupChecks: %v", err)
	}
	schema := startupCheckByName(checks, "config-schema")
	if schema == nil {
		t.Fatalf("missing config-schema warning in %#v", checks)
	}
	if schema.Status != startupWarn {
		t.Fatalf("expected config-schema warn, got %s", schema.Status)
	}
}

func TestResolveHTTPPortPriority(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("HTTP_PORT=9123\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("HTTP_PORT", "7777")
	if got := resolveHTTPPort(root); got != "7777" {
		t.Fatalf("expected env HTTP_PORT to win, got %q", got)
	}

	t.Setenv("HTTP_PORT", "")
	if got := resolveHTTPPort(root); got != "9123" {
		t.Fatalf("expected .env HTTP_PORT, got %q", got)
	}

	if err := os.Remove(filepath.Join(root, ".env")); err != nil {
		t.Fatalf("remove .env: %v", err)
	}
	if got := resolveHTTPPort(root); got != "8080" {
		t.Fatalf("expected default 8080, got %q", got)
	}
}

func TestSuggestPortConflictFindsAlternative(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	parts := strings.Split(addr, ":")
	busyPort := parts[len(parts)-1]
	if _, err := strconv.Atoi(busyPort); err != nil {
		t.Fatalf("invalid busy port %q from addr %q", busyPort, addr)
	}

	t.Setenv("HTTP_PORT", busyPort)
	used, alt := suggestPortConflict(t.TempDir())
	if used != busyPort {
		t.Fatalf("expected used port %s, got %s", busyPort, used)
	}
	if alt == "" {
		t.Fatalf("expected alternative port for busy port %s", busyPort)
	}
	if alt == busyPort {
		t.Fatalf("expected different alternative port, got same %s", alt)
	}
}

func TestRunSmokeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/health/ready" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	runSmoke([]string{"--base-url", srv.URL, "--timeout", "1s"})
}

func TestRunSmokeFailureExitsNonZero(t *testing.T) {
	if os.Getenv("ANG_TEST_SMOKE_HELPER") == "1" {
		runSmoke([]string{"--base-url", os.Getenv("ANG_TEST_SMOKE_URL"), "--timeout", "1s"})
		return
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
	}))
	defer srv.Close()

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunSmokeFailureExitsNonZero$")
	cmd.Env = append(os.Environ(), "ANG_TEST_SMOKE_HELPER=1", "ANG_TEST_SMOKE_URL="+srv.URL)

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected runSmoke helper process to exit non-zero")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exit error, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
	}
}

func startupCheckByName(checks []startupCheck, name string) *startupCheck {
	for i := range checks {
		if checks[i].Name == name {
			return &checks[i]
		}
	}
	return nil
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}
