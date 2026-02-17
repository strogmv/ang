package mcp

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func FuzzNormalizeToolResultEnvelope(f *testing.F) {
	f.Add([]byte(`{"ok":true}`))
	f.Add([]byte(`not-json`))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		env := func(name, status string, payload any, extra map[string]any) *mcp.CallToolResult {
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
		resp := mcp.NewToolResultText(string(data))
		out := normalizeToolResultEnvelope("fuzz_tool", resp, env)
		if out == nil {
			t.Fatalf("normalize returned nil")
		}
	})
}

func FuzzSafeInvokeTool_NoCrash(f *testing.F) {
	f.Add("boom")
	f.Add("")
	f.Add("panic-with-unicode-\u2603")

	f.Fuzz(func(t *testing.T, panicMessage string) {
		env := func(name, status string, payload any, extra map[string]any) *mcp.CallToolResult {
			b, _ := json.Marshal(map[string]any{
				"tool":    name,
				"status":  status,
				"payload": payload,
			})
			return mcp.NewToolResultText(string(b))
		}
		resp, err := safeInvokeTool("fuzz_tool", true, env, func() (*mcp.CallToolResult, error) {
			panic(panicMessage)
		})
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}
		if resp == nil {
			t.Fatalf("expected response")
		}
	})
}
