package mcp

import (
	"encoding/json"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

const maxNormalizedTextPayloadBytes = 1 << 20 // 1 MiB

// normalizeToolResultEnvelope converts arbitrary MCP tool result into a stable envelope payload.
// It is defensive by design and must never panic.
func normalizeToolResultEnvelope(name string, resp *mcp.CallToolResult, envFn envelopeFunc) *mcp.CallToolResult {
	if envFn == nil {
		if resp == nil {
			return mcp.NewToolResultText(`{"status":"ok","payload":{}}`)
		}
		return resp
	}

	status := "ok"
	if resp != nil && resp.IsError {
		status = "tool_error"
	}
	if resp == nil {
		if wrapped, ok := safeEnvelopeCall(envFn, name, status, map[string]any{}, nil); ok && wrapped != nil {
			return wrapped
		}
		return mcp.NewToolResultText(`{"status":"ok","payload":{}}`)
	}
	if resp.StructuredContent != nil {
		if wrapped, ok := safeEnvelopeCall(envFn, name, status, resp.StructuredContent, nil); ok && wrapped != nil {
			return wrapped
		}
		return mcp.NewToolResultText(`{"status":"ok","payload":{"message":"envelope unavailable"}}`)
	}

	text := firstTextContent(resp)
	if strings.TrimSpace(text) == "" {
		if wrapped, ok := safeEnvelopeCall(envFn, name, status, map[string]any{"note": "non-text MCP content"}, nil); ok && wrapped != nil {
			return wrapped
		}
		return mcp.NewToolResultText(`{"status":"ok","payload":{"note":"non-text MCP content"}}`)
	}

	extra := map[string]any{}
	if len(text) > maxNormalizedTextPayloadBytes {
		text = text[:maxNormalizedTextPayloadBytes]
		extra["payload_truncated"] = true
	}

	var parsed any
	if json.Unmarshal([]byte(text), &parsed) == nil {
		if wrapped, ok := safeEnvelopeCall(envFn, name, status, parsed, extra); ok && wrapped != nil {
			return wrapped
		}
		return mcp.NewToolResultText(`{"status":"ok","payload":{"message":"envelope unavailable"}}`)
	}
	if wrapped, ok := safeEnvelopeCall(envFn, name, status, map[string]any{"message": text}, extra); ok && wrapped != nil {
		return wrapped
	}
	return mcp.NewToolResultText(`{"status":"ok","payload":{"message":"envelope unavailable"}}`)
}

func firstTextContent(resp *mcp.CallToolResult) string {
	if resp == nil {
		return ""
	}
	for _, c := range resp.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
