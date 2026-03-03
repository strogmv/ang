package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolveStatusBaseURL_Default(t *testing.T) {
	t.Setenv("HTTP_PORT", "9099")
	got := resolveStatusBaseURL(t.TempDir(), "")
	if got != "http://localhost:9099" {
		t.Fatalf("expected default base url, got %q", got)
	}
}

func TestResolveStatusBaseURL_Explicit(t *testing.T) {
	got := resolveStatusBaseURL(t.TempDir(), "http://127.0.0.1:18080/")
	if got != "http://127.0.0.1:18080" {
		t.Fatalf("expected trimmed explicit base url, got %q", got)
	}
}

func TestProbeHealthEndpoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/health/ready":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	health, ready := probeHealthEndpoints(srv.URL, 1*time.Second)
	if health != "OK" {
		t.Fatalf("expected health OK, got %q", health)
	}
	if ready == "OK" {
		t.Fatalf("expected ready DOWN, got %q", ready)
	}
}

func TestNonEmptyLines(t *testing.T) {
	got := nonEmptyLines("\n a \n\nb\n")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected parsed lines: %#v", got)
	}
}
