package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	Rationale   string               `json:"rationale,omitempty"`
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

	// --- AI HEALER (Stage 32) ---

	s.AddTool(mcp.NewTool("ang_doctor",
		mcp.WithDescription("Analyze build/test logs and suggest CUE-level fixes for errors"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logData, _ := os.ReadFile("ang-build.log")
		log := string(logData)

		report := &ANGReport{
			Status:  "Analyzing",
			Summary: []string{"Healer analysis of ang-build.log"},
		}

		// Pattern matching for typical errors
		if strings.Contains(log, "range can't iterate over") {
			report.Summary = append(report.Summary, "Found common logic.Call list error.")
			report.Diagnostics = append(report.Diagnostics, normalizer.Warning{
				Kind: "template",
				Code: "LIST_REQUIRED",
				Message: "Arguments for logic.Call must be a CUE list [\"arg\"], not a single value.",
				Severity: "error",
				Hint: "Wrap the argument in brackets in your CUE file.",
				CanAutoApply: true,
			})
			report.NextActions = append(report.NextActions, "cue_apply_patch with bracket fix", "run_preset('build')")
			report.Rationale = "The Go template engine expects a slice for method arguments iteration."
		}

		if strings.Contains(log, "ARCHITECTURE WARNING") {
			report.Summary = append(report.Summary, "Found ownership violation.")
			report.Diagnostics = append(report.Diagnostics, normalizer.Warning{
				Kind: "architecture",
				Code: "OWNERSHIP_VIOLATION",
				Message: "Service is accessing entity it doesn't own.",
				Hint: "Add @owner() attribute or move the operation.",
			})
		}

		if len(report.Diagnostics) == 0 {
			report.Status = "Healthy"
			report.Summary = append(report.Summary, "No obvious patterns found in logs. Check manual fixes.")
		}

		return mcp.NewToolResultText(report.ToJSON()), nil
	})

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
				"bug_fix":     []string{"run_preset('unit')", "ang_doctor", "cue_apply_patch", "run_preset('build')"},
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
		cmd := exec.Command("grep", "-r", "-n", "-i", query, "cue/", "internal/")
		out, _ := cmd.CombinedOutput()
		return mcp.NewToolResultText(string(out)), nil
	})

	s.AddTool(mcp.NewTool("cue_apply_patch",
		mcp.WithDescription("Update CUE intent with atomic validation (syntax + architecture)"),
		mcp.WithString("path", mcp.Description("CUE file path"), mcp.Required()),
		mcp.WithString("content", mcp.Description("CUE patch content"), mcp.Required()),
		mcp.WithString("selector", mcp.Description("Target node path")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, content := mcp.ParseString(request, "path", ""), mcp.ParseString(request, "content", "")
		selector := mcp.ParseString(request, "selector", "")
		if !strings.HasPrefix(path, "cue/") { return mcp.NewToolResultText("Denied: only /cue directory is modifiable"), nil }
		
		// 1. Get Merged Content
		newContent, err := GetMergedContent(path, selector, content)
		if err != nil { return mcp.NewToolResultText(fmt.Sprintf("Merge error: %v", err)), nil }

		// 2. Backup & Apply (Temporary)
		orig, _ := os.ReadFile(path)
		os.WriteFile(path, newContent, 0644)

		// 3. Syntax Check (cue vet on package)
		dir := filepath.Dir(path)
		cmd := exec.Command("cue", "vet", "./" + dir) // Run on the whole directory
		if out, err := cmd.CombinedOutput(); err != nil {
			os.WriteFile(path, orig, 0644) // Rollback
			return mcp.NewToolResultText(fmt.Sprintf("Syntax validation FAILED (context: %s):\n%s", dir, string(out))), nil
		}

		// 4. Architectural Check (ang validate)
		if _, _, _, _, _, _, _, _, err := compiler.RunPipeline("."); err != nil {
			os.WriteFile(path, orig, 0644) // Rollback
			return mcp.NewToolResultText(fmt.Sprintf("Architecture validation FAILED: %v", err)), nil
		}

		sessionState.Lock()
		sessionState.LastAction = "cue_apply_patch"
		sessionState.Unlock()
		return mcp.NewToolResultText("Intent merged and validated successfully. Next: run_preset('build')"), nil
	})

	s.AddTool(mcp.NewTool("run_preset",
		mcp.WithDescription("Run safe workflow commands: build, unit, lint"),
		mcp.WithString("name", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")
		var cmd *exec.Cmd
		switch name {
		case "build": cmd = exec.Command("./ang_bin", "build")
		case "unit":  cmd = exec.Command("go", "test", "-v", "./...")
		default: return mcp.NewToolResultText("Unknown preset"), nil
		}
		logFile := "ang-build.log"
		f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		cmd.Stdout, cmd.Stderr = f, f
		err := cmd.Run()
		f.Close()
		status := "SUCCESS"
		if err != nil { status = "FAILED" }
		return mcp.NewToolResultText(fmt.Sprintf("Preset %s finished: %s. See resource://ang/logs/build", name, status)), nil
	})

	registerResources(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func registerResources(s *server.MCPServer) {
	s.AddResource(mcp.NewResource("resource://ang/logs/build", "Live Build Log", mcp.WithMIMEType("text/plain")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			content, _ := os.ReadFile("ang-build.log")
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/logs/build", MIMEType: "text/plain", Text: string(content)}}, nil
		})

	s.AddResource(mcp.NewResource("resource://ang/policy", "AI Policy", mcp.WithMIMEType("text/plain")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			policy := "1. Agent writes ONLY CUE.\n2. ANG generates ALL code.\n3. NEVER touch .go files manually."
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/policy", MIMEType: "text/plain", Text: policy}}, nil
		})
}
