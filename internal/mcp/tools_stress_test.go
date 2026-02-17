package mcp

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPStress_RandomCallSequence(t *testing.T) {
	tools := registerStressToolSet()
	if len(tools) == 0 {
		t.Fatal("expected non-empty tool set")
	}

	r := rand.New(rand.NewSource(42))
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}

	for i := 0; i < 250; i++ {
		toolName := names[r.Intn(len(names))]
		req := randomStressRequest(r, toolName)
		resp, err := invokeToolLikeServer(toolName, tools[toolName], req)
		if err != nil {
			t.Fatalf("iteration %d tool %s: unexpected error: %v", i, toolName, err)
		}
		if resp == nil {
			t.Fatalf("iteration %d tool %s: nil response", i, toolName)
		}
		if len(resp.Content) == 0 {
			t.Fatalf("iteration %d tool %s: empty content", i, toolName)
		}
		tc, ok := resp.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatalf("iteration %d tool %s: expected text content, got %#v", i, toolName, resp.Content[0])
		}
		if strings.TrimSpace(tc.Text) == "" {
			t.Fatalf("iteration %d tool %s: blank payload", i, toolName)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &parsed); err != nil {
			t.Fatalf("iteration %d tool %s: response is not valid json: %v\npayload=%s", i, toolName, err, tc.Text)
		}
		if parsed["status"] == nil {
			t.Fatalf("iteration %d tool %s: envelope status missing in %v", i, toolName, parsed)
		}
	}
}

func registerStressToolSet() map[string]func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	allowed := map[string]bool{
		"ang_mcp_health":        true,
		"ang_schema":            true,
		"ang_diff_architecture": true,
		"ang_search":            true,
		"repo_read_symbol":      true,
		"ang_plan":              true,
		"ang_doctor":            true,
		"ang_validate_logic":    true,
	}

	tools := map[string]func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error){}
	addTool := func(name string, tool mcp.Tool, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
		if allowed[name] {
			tools[name] = h
		}
	}

	registerCoreTools(addTool, coreToolDeps{
		currentProfile:     func() string { return "compact" },
		runtimeConfigPath:  func() string { return ".ang/mcp-runtime.json" },
		runtimeConfigError: func() string { return "" },
		featureAddWorkflow: func() []string { return []string{} },
		bugFixWorkflow:     func() []string { return []string{} },
		bootstrapExempt:    func() map[string]bool { return map[string]bool{} },
		envelopeEnabled:    func() bool { return true },
		searchLimits:       func() (int, int) { return 25, 80 },
		symbolLimits:       func() (int, int) { return 6, 24 },
		snapshotLimits:     func() (int, int) { return 25, 80 },
		mcpSchemaVersion:   "mcp-envelope/v1",
	})
	registerPlanTools(addTool)
	registerAnalysisTools(addTool)

	return tools
}

func invokeToolLikeServer(name string, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	return safeInvokeTool(name, true, env, func() (*mcp.CallToolResult, error) {
		resp, err := h(context.Background(), request)
		if err != nil {
			return env(name, "tool_error", map[string]any{"message": err.Error()}, nil), nil
		}
		return normalizeToolResultEnvelope(name, resp, env), nil
	})
}

func randomStressRequest(r *rand.Rand, toolName string) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	args := map[string]any{
		"goal":           randomValue(r, 0),
		"base_ref":       randomValue(r, 0),
		"query":          randomValue(r, 0),
		"path":           randomValue(r, 0),
		"symbol":         randomValue(r, 0),
		"entity":         randomValue(r, 0),
		"service":        randomValue(r, 0),
		"endpoint":       randomValue(r, 0),
		"scope":          randomValue(r, 0),
		"max_lines":      randomValue(r, 0),
		"context":        randomValue(r, 0),
		"include_fields": randomValue(r, 0),
		"run_tests":      randomValue(r, 0),
		"run_go_build":   randomValue(r, 0),
	}
	req.Params.Arguments = args
	return req
}

func randomValue(r *rand.Rand, depth int) any {
	if depth > 2 {
		return r.Intn(10)
	}
	switch r.Intn(9) {
	case 0:
		return nil
	case 1:
		return r.Intn(2) == 0
	case 2:
		return r.Float64() * 1000
	case 3:
		return r.Intn(1000)
	case 4:
		return strings.Repeat("x", r.Intn(128))
	case 5:
		return time.Unix(int64(r.Intn(1_000_000)), 0).UTC().Format(time.RFC3339)
	case 6:
		return []any{randomValue(r, depth+1), randomValue(r, depth+1)}
	case 7:
		return map[string]any{
			"a": randomValue(r, depth+1),
			"b": randomValue(r, depth+1),
		}
	default:
		return "val"
	}
}
