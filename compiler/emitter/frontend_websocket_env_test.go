package emitter

import (
	"os"
	"strings"
	"testing"
)

// Vite substitutes import.meta.env only when it is written out directly. The
// socket base URL once read it through a variable, which left production
// sockets dialling ws://localhost:8080; this keeps it from coming back.
func TestWebSocketTemplateReadsViteEnvDirectly(t *testing.T) {
	raw, err := os.ReadFile("../../templates/frontend/websocket.ts.tmpl")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	src := string(raw)
	start := strings.Index(src, "export const getWebSocketBaseUrl")
	if start < 0 {
		t.Fatal("getWebSocketBaseUrl not found in websocket.ts.tmpl")
	}
	body := src[start:]
	if end := strings.Index(body, "\n};"); end > 0 {
		body = body[:end]
	}
	if !strings.Contains(body, "import.meta.env") {
		t.Errorf("getWebSocketBaseUrl must read import.meta.env directly so Vite can replace it:\n%s", body)
	}
	if strings.Contains(body, "= import.meta as") || strings.Contains(body, "= import.meta;") {
		t.Errorf("getWebSocketBaseUrl reads import.meta through a variable; Vite will not replace it:\n%s", body)
	}
}
