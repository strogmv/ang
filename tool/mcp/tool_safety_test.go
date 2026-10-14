package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestSafeInvokeTool_PanicWithEnvelope(t *testing.T) {
	env := func(name, status string, payload any, extra map[string]any) *mcp.CallToolResult {
		body := map[string]any{
			"tool":    name,
			"status":  status,
			"payload": payload,
		}
		b, _ := json.Marshal(body)
		return mcp.NewToolResultText(string(b))
	}

	resp, err := safeInvokeTool("ang_schema", true, env, func() (*mcp.CallToolResult, error) {
		panic("boom")
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if len(resp.Content) == 0 {
		t.Fatal("expected response content")
	}
	tc, ok := resp.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %#v", resp.Content[0])
	}
	if !strings.Contains(tc.Text, `"status":"tool_error"`) {
		t.Fatalf("expected tool_error status, got %s", tc.Text)
	}
	if !strings.Contains(tc.Text, "tool panic: boom") {
		t.Fatalf("expected panic message, got %s", tc.Text)
	}
}

func TestSafeInvokeTool_PanicWithoutEnvelope(t *testing.T) {
	resp, err := safeInvokeTool("ang_schema", false, nil, func() (*mcp.CallToolResult, error) {
		panic("boom-no-envelope")
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	tc, ok := resp.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %#v", resp.Content[0])
	}
	if !strings.Contains(tc.Text, "tool panic: boom-no-envelope") {
		t.Fatalf("unexpected panic text: %s", tc.Text)
	}
}

func TestSafeInvokeTool_NoPanic(t *testing.T) {
	want := mcp.NewToolResultText(`{"ok":true}`)
	resp, err := safeInvokeTool("ang_schema", true, nil, func() (*mcp.CallToolResult, error) {
		return want, nil
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatal("expected non-empty response")
	}
	tc, ok := resp.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %#v", resp.Content[0])
	}
	if tc.Text != `{"ok":true}` {
		t.Fatalf("unexpected response text: %s", tc.Text)
	}
}
