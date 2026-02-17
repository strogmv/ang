package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func testEnvelope(name, status string, payload any, extra map[string]any) *mcp.CallToolResult {
	body := map[string]any{
		"tool":    name,
		"status":  status,
		"payload": payload,
	}
	for k, v := range extra {
		body[k] = v
	}
	b, _ := json.Marshal(body)
	return mcp.NewToolResultText(string(b))
}

func TestNormalizeToolResultEnvelope_TruncatesLargePayload(t *testing.T) {
	large := strings.Repeat("a", maxNormalizedTextPayloadBytes+64)
	resp := mcp.NewToolResultText(large)
	out := normalizeToolResultEnvelope("ang_schema", resp, testEnvelope)
	if out == nil || len(out.Content) == 0 {
		t.Fatal("expected output content")
	}
	tc, ok := out.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %#v", out.Content[0])
	}
	if !strings.Contains(tc.Text, `"payload_truncated":true`) {
		t.Fatalf("expected payload_truncated flag, got: %s", tc.Text)
	}
}

func TestSafeInvokeTool_EnvelopePanicFallback(t *testing.T) {
	panicEnv := func(name, status string, payload any, extra map[string]any) *mcp.CallToolResult {
		panic("envelope boom")
	}
	resp, err := safeInvokeTool("ang_schema", true, panicEnv, func() (*mcp.CallToolResult, error) {
		panic("tool boom")
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatal("expected fallback response")
	}
	tc, ok := resp.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text response, got %#v", resp.Content[0])
	}
	if !strings.Contains(tc.Text, "tool panic: tool boom") {
		t.Fatalf("expected panic fallback message, got: %s", tc.Text)
	}
}
