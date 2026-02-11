package mcp

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
)

type envelopeFunc func(name, status string, payload any, extra map[string]any) *mcp.CallToolResult

// safeInvokeTool ensures panics from tool handlers are converted into tool results
// instead of crashing the MCP process and closing transport.
func safeInvokeTool(name string, envelopeEnabled bool, envFn envelopeFunc, h func() (*mcp.CallToolResult, error)) (resp *mcp.CallToolResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("tool panic: %v", r)
			fmt.Fprintln(os.Stderr, "[ANG MCP] "+msg)
			if envelopeEnabled && envFn != nil {
				resp = envFn(name, "tool_error", map[string]any{
					"message": msg,
				}, nil)
				err = nil
				return
			}
			resp = mcp.NewToolResultText(msg)
			err = nil
		}
	}()
	return h()
}
