package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestEmitPythonSDK_IncludesAsyncAndRFC9457(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := wd
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatalf("repo root with go.mod not found from %s", wd)
		}
		root = parent
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	em.Version = "0.1.0"

	endpoints := []normalizer.Endpoint{
		{Method: "POST", Path: "/auth/login", ServiceName: "Auth", RPC: "Login"},
	}
	services := []normalizer.Service{
		{
			Name: "Auth",
			Methods: []normalizer.Method{
				{
					Name:   "Login",
					Input:  normalizer.Entity{Name: "LoginRequest"},
					Output: normalizer.Entity{Name: "AuthTokens"},
				},
			},
		},
	}
	entities := []normalizer.Entity{
		{Name: "LoginRequest", Fields: []normalizer.Field{{Name: "email", Type: "string"}}},
		{Name: "AuthTokens", Fields: []normalizer.Field{{Name: "accessToken", Type: "string"}}},
	}

	if err := em.EmitPythonSDK(endpoints, services, entities, nil); err != nil {
		t.Fatalf("emit python sdk: %v", err)
	}

	clientPath := filepath.Join(tmp, "sdk", "python", "ang_sdk", "client.py")
	clientData, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read client.py: %v", err)
	}
	clientText := string(clientData)
	if !strings.Contains(clientText, "class AsyncAngClient") {
		t.Fatalf("expected async client in client.py")
	}
	if !strings.Contains(clientText, "ProblemDetails") || !strings.Contains(clientText, "AngAPIError") {
		t.Fatalf("expected RFC9457 error mapping hooks in client.py")
	}
	if !strings.Contains(clientText, "models.AuthTokens.model_validate") {
		t.Fatalf("expected typed response validation in client.py")
	}

	errorsPath := filepath.Join(tmp, "sdk", "python", "ang_sdk", "errors.py")
	errorsData, err := os.ReadFile(errorsPath)
	if err != nil {
		t.Fatalf("read errors.py: %v", err)
	}
	errorsText := string(errorsData)
	if !strings.Contains(errorsText, "class ProblemDetails") || !strings.Contains(errorsText, "class AngAPIError") {
		t.Fatalf("expected ProblemDetails/AngAPIError in errors.py")
	}
}
