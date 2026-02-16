package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/ir"
)

func TestEmitFrontendSDK_GeneratesErrorNormalizerModule(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New("", tmp, "templates")
	em.Version = "0.1.0"

	services := []ir.Service{
		{
			Name: "Auth",
			Methods: []ir.Method{
				{
					Name:  "Login",
					Input: &ir.Entity{Name: "LoginRequest"},
					Output: &ir.Entity{
						Name: "AuthTokens",
					},
				},
			},
		},
	}
	endpoints := []ir.Endpoint{
		{
			Method:  "POST",
			Path:    "/api/auth/login",
			Service: "Auth",
			RPC:     "Login",
		},
	}

	if err := em.EmitFrontendSDK(nil, services, endpoints, nil, nil, nil); err != nil {
		t.Fatalf("emit frontend sdk: %v", err)
	}

	normalizerPath := filepath.Join(tmp, "error-normalizer.ts")
	normalizerData, err := os.ReadFile(normalizerPath)
	if err != nil {
		t.Fatalf("read generated error-normalizer.ts: %v", err)
	}
	normalizerText := string(normalizerData)
	if !strings.Contains(normalizerText, "normalizeApiError") {
		t.Fatalf("expected normalizeApiError export in error-normalizer.ts")
	}
	if !strings.Contains(normalizerText, "normalizeProblemDetail") {
		t.Fatalf("expected normalizeProblemDetail export in error-normalizer.ts")
	}

	clientPath := filepath.Join(tmp, "api-client.ts")
	clientData, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read generated api-client.ts: %v", err)
	}
	clientText := string(clientData)
	if !strings.Contains(clientText, "from './error-normalizer'") {
		t.Fatalf("expected api-client.ts to import error-normalizer module")
	}
	if !strings.Contains(clientText, "normalizeApiError({") {
		t.Fatalf("expected api-client.ts to use normalizeApiError")
	}
}
