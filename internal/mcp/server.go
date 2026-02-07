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
				"feature_add": []string{"ang_plan", "ang_search", "cue_apply_patch", "run_preset('build')", "run_preset('unit')"},
				"bug_fix":     []string{"run_preset('unit')", "ang_search", "cue_apply_patch", "run_preset('build')"},
			},
			"resources": []string{
				"resource://ang/logs/build",
				"resource://ang/policy",
				"resource://ang/readme_for_agents",
			},
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// --- ESSENTIAL TOOLS ---

	s.AddTool(mcp.NewTool("ang_search",
		mcp.WithDescription("Hybrid symbol search. Use this to find where logic is implemented."),
		mcp.WithString("query", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := mcp.ParseString(request, "query", "")
		// Simple grep for MVP
		cmd := exec.Command("grep", "-r", "-n", "-i", query, "cue/", "internal/")
		out, _ := cmd.CombinedOutput()
		return mcp.NewToolResultText(string(out)), nil
	})

	s.AddTool(mcp.NewTool("repo_diff",
		mcp.WithDescription("See how the code changed after generation. Token-efficient."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("git", "diff")
		out, _ := cmd.CombinedOutput()
		return mcp.NewToolResultText(string(out)), nil
	})

	s.AddTool(mcp.NewTool("cue_apply_patch",
		mcp.WithDescription("Update CUE intent. ONLY allowed way to modify the system."),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("content", mcp.Required()),
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
		mcp.WithDescription("Run safe workflow commands: build, unit, lint"),
		mcp.WithString("name", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")
		var cmd *exec.Cmd
		switch name {
		case "build": 
			cmd = exec.Command("./ang_bin", "build")
		case "unit":  
			cmd = exec.Command("go", "test", "-v", "./...")
		default: 
			return mcp.NewToolResultText("Unknown preset"), nil
		}
		
		// Redirect output to log file for "Live Log" resource
		logFile := "ang-build.log"
		f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		cmd.Stdout = f
		cmd.Stderr = f
		
		err := cmd.Run()
		f.Close()

		if name == "build" {
			sessionState.Lock()
			sessionState.LastAction = "build"
			sessionState.Unlock()
		}

		status := "SUCCESS"
		if err != nil { status = "FAILED" }
		
		return mcp.NewToolResultText(fmt.Sprintf("Preset %s finished with status: %s. Check resource://ang/logs/build for details.", name, status)), nil
	})

	// --- RESOURCES ---

	s.AddResource(mcp.NewResource("resource://ang/logs/build", "Live Build Log", mcp.WithMIMEType("text/plain")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			content, err := os.ReadFile("ang-build.log")
			if err != nil { return nil, fmt.Errorf("log file not found (run build first)") }
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/logs/build", MIMEType: "text/plain", Text: string(content)}}, nil
		})

	s.AddResource(mcp.NewResource("resource://ang/policy", "AI Policy", mcp.WithMIMEType("text/plain")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			policy := "1. Agent writes ONLY CUE.\n2. ANG generates ALL code.\n3. NEVER touch .go files manually."
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/policy", MIMEType: "text/plain", Text: policy}}, nil
		})

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
