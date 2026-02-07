package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
)

var (
	projectLocks sync.Map // map[string]*sync.Mutex
	sessionState = struct {
		sync.Mutex
		LastAction string
	}{}
)

func getLock(path string) *sync.Mutex {
	lock, _ := projectLocks.LoadOrStore(path, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// ANGReport - Unified response structure
type ANGReport struct {
	Status      string               `json:"status"`
	Summary     []string             `json:"summary"`
	Diagnostics []normalizer.Warning `json:"diagnostics,omitempty"`
	NextActions []string             `json:"next_actions,omitempty"`
	Artifacts   map[string]string    `json:"artifacts,omitempty"`
}

func (r *ANGReport) ToJSON() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

func Run() {
	s := server.NewMCPServer(
		"ANG MCP Server",
		compiler.Version,
		server.WithLogging(),
	)

	// --- MANDATORY ENTRYPOINT ---

	s.AddTool(mcp.NewTool("ang_bootstrap",
		mcp.WithDescription("Mandatory first step. Detects project and returns AI-optimized workflows."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cwd, _ := os.Getwd()
		res := map[string]interface{}{
			"status":           "Ready",
			"project_detected": true,
			"repo_root":        cwd,
			"ang_version":      compiler.Version,
			"policy":           "Agent writes only CUE. ANG writes code. Agent reads code and runs tests.",
			"workflows": map[string]interface{}{
				"feature_add": []string{"ang_plan", "cue_read", "cue_apply_patch", "run_preset('build')", "run_preset('unit')"},
				"bug_fix":     []string{"run_preset('unit')", "ang_flow_query", "cue_apply_patch", "run_preset('build')"},
			},
			"next_steps": []string{"Call ang_plan with your goal", "Read resource://ang/readme_for_agents"},
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// --- FLOW CONTROL ---

	s.AddTool(mcp.NewTool("ang_next",
		mcp.WithDescription("Get the next logical step based on current project state"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionState.Lock()
		last := sessionState.LastAction
		sessionState.Unlock()

		suggestion := "Call ang_bootstrap to start"
		if last == "cue_apply_patch" {
			suggestion = "Call run_preset('build') to generate code from your changes"
		} else if last == "build" {
			suggestion = "Call run_preset('unit') to verify the generated code"
		}

		return mcp.NewToolResultText(fmt.Sprintf("Suggested next step: %s", suggestion)), nil
	})

	// --- ESSENTIAL TOOLS ---

	s.AddTool(mcp.NewTool("ang_plan",
		mcp.WithDescription("Generate a structured step-by-step plan for your goal"),
		mcp.WithString("goal", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		goal := mcp.ParseString(request, "goal", "")
		report := &ANGReport{
			Status: "Planning",
			Summary: []string{"Goal: " + goal},
			NextActions: []string{"1. ang_search for relevant logic", "2. cue_apply_patch to update intent", "3. run_preset('build')"},
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	s.AddTool(mcp.NewTool("ang_validate",
		mcp.WithDescription("Validate CUE intent and check for architectural violations"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, _, _, _, _, _, _, err := compiler.RunPipeline(".")
		report := &ANGReport{
			Status: "Validated",
			Diagnostics: compiler.LatestDiagnostics,
		}
		if err != nil { report.Status = "Invalid"; report.Summary = append(report.Summary, err.Error()) }
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	s.AddTool(mcp.NewTool("cue_apply_patch",
		mcp.WithDescription("Update CUE intent. ONLY allowed way to modify the system."),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("content", mcp.Required()),
		mcp.WithBoolean("dry_run"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, content := mcp.ParseString(request, "path", ""), mcp.ParseString(request, "content", "")
		if !strings.HasPrefix(path, "cue/") { return mcp.NewToolResultText("Denied: only /cue directory is modifiable"), nil }
		
		if err := os.WriteFile(path, []byte(content), 0644); err != nil { return mcp.NewToolResultText(err.Error()), nil }
		
		sessionState.Lock()
		sessionState.LastAction = "cue_apply_patch"
		sessionState.Unlock()

		return mcp.NewToolResultText("Intent updated. Next: run_preset('build')"), nil
	})

	s.AddTool(mcp.NewTool("run_preset",
		mcp.WithDescription("Run safe workflow commands: build, unit, lint, migrate"),
		mcp.WithString("name", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")
		var cmd *exec.Cmd
		switch name {
		case "build": cmd = exec.Command("./ang_bin", "build")
		case "unit":  cmd = exec.Command("go", "test", "./...")
		default: return mcp.NewToolResultText("Unknown preset"), nil
		}
		
		out, _ := cmd.CombinedOutput()
		if name == "build" {
			sessionState.Lock()
			sessionState.LastAction = "build"
			sessionState.Unlock()
		}
		return mcp.NewToolResultText(string(out)), nil
	})

	// --- RESOURCES ---

	s.AddResource(mcp.NewResource("resource://ang/policy", "AI Policy", mcp.WithMIMEType("text/plain")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			policy := "1. Agent writes ONLY CUE.\n2. ANG generates ALL code.\n3. NEVER touch .go files manually.\n4. Use repo_diff to see changes."
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/policy", MIMEType: "text/plain", Text: policy}}, nil
		})

	s.AddResource(mcp.NewResource("resource://ang/readme_for_agents", "Readme for AI", mcp.WithMIMEType("text/plain")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			readme := "ANG is an Intent-First compiler. Your job is to architect the CUE models. ANG takes care of the rest."
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/readme_for_agents", MIMEType: "text/plain", Text: readme}}, nil
		})

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}