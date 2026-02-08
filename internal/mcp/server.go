package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	Impacts     []string             `json:"impacts,omitempty"`
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

	// --- DB SYNC (Stage 41) ---

	s.AddTool(mcp.NewTool("ang_db_sync",
		mcp.WithDescription("Synchronize database schema with current CUE intent (requires DATABASE_URL)"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("./ang_bin", "db", "sync")
		out, err := cmd.CombinedOutput()
		status := "Success"
		if err != nil { status = "Failed" }
		report := &ANGReport{
			Status: status,
			Summary: []string{"Database synchronization results"},
			Artifacts: map[string]string{"log": string(out)},
			Rationale: "Ensures the physical database schema matches your CUE domain models.",
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- RBAC OBSERVABILITY (Stage 40) ---

	s.AddTool(mcp.NewTool("ang_list_actions",
		mcp.WithDescription("List all available RBAC actions in the system (service.method)"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, services, _, _, _, _, _, _, err := compiler.RunPipeline(".")
		if err != nil { return mcp.NewToolResultText(err.Error()), nil }
		var actions []string
		for _, s := range services {
			for _, m := range s.Methods {
				actions = append(actions, fmt.Sprintf("%s.%s", strings.ToLower(s.Name), strings.ToLower(m.Name)))
			}
		}
		report := &ANGReport{ Status: "Success", Impacts: actions }
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- INTENT DEBUGGER (Stage 39) ---

	s.AddTool(mcp.NewTool("ang_explain_error",
		mcp.WithDescription("Map runtime error back to CUE intent and explain what went wrong"),
		mcp.WithString("log", mcp.Description("Raw error log or JSON"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		log := mcp.ParseString(request, "log", "")
		re := regexp.MustCompile(`"intent":\s*"([^"]+):(\d+)(?:\s*\(([^)]+)\))?"`)
		matches := re.FindStringSubmatch(log)
		if len(matches) < 3 {
			return mcp.NewToolResultText("No intent metadata found in log."), nil
		}
		file, line := matches[1], matches[2]
		content, _ := os.ReadFile(file)
		lines := strings.Split(string(content), "\n")
		snippet := ""
		lineIdx := 0
		fmt.Sscanf(line, "%d", &lineIdx)
		if lineIdx > 0 && lineIdx <= len(lines) {
			start := lineIdx - 3
			if start < 0 { start = 0 }
			end := lineIdx + 2
			if end > len(lines) { end = len(lines) }
			snippet = strings.Join(lines[start:end], "\n")
		}
		report := &ANGReport{
			Status: "Debugging",
			Summary: []string{fmt.Sprintf("Error mapped to %s:%s", file, line)},
			Artifacts: map[string]string{"cue_snippet": snippet},
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- AI HEALER ---

	s.AddTool(mcp.NewTool("ang_doctor",
		mcp.WithDescription("Analyze build/test logs and suggest CUE-level fixes"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logData, _ := os.ReadFile("ang-build.log")
		log := string(logData)
		report := &ANGReport{ Status: "Analyzing", Summary: []string{"Healer analysis"} }
		if strings.Contains(log, "range can't iterate over") {
			report.Diagnostics = append(report.Diagnostics, normalizer.Warning{
				Kind: "template", Code: "LIST_REQUIRED", Message: "logic.Call args must be a list.", CanAutoApply: true,
			})
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- MANDATORY ENTRYPOINT ---

	s.AddTool(mcp.NewTool("ang_bootstrap",
		mcp.WithDescription("Mandatory first step."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res := map[string]interface{}{
			"status": "Ready",
			"ang_version": compiler.Version,
			"workflows": map[string]interface{}{
				"feature_add": []string{"ang_plan", "ang_search", "ang_list_actions", "cue_apply_patch", "run_preset('build')", "ang_db_sync"},
				"bug_fix":     []string{"run_preset('unit')", "ang_explain_error", "ang_doctor", "cue_apply_patch", "run_preset('build')"},
			},
			"resources": []string{"resource://ang/logs/build", "resource://ang/policy"},
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// --- ESSENTIAL TOOLS ---

	s.AddTool(mcp.NewTool("ang_search",
		mcp.WithDescription("Hybrid symbol search."),
		mcp.WithString("query", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := mcp.ParseString(request, "query", "")
		cmd := exec.Command("grep", "-r", "-n", "-i", query, "cue/", "internal/")
		out, _ := cmd.CombinedOutput()
		return mcp.NewToolResultText(string(out)), nil
	})

	s.AddTool(mcp.NewTool("cue_apply_patch",
		mcp.WithDescription("Update CUE intent with atomic validation"),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("content", mcp.Required()),
		mcp.WithString("selector", mcp.Description("Target node path")),
		mcp.WithBoolean("forced_merge", mcp.Description("Overwrite instead of deep merge")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, content := mcp.ParseString(request, "path", ""), mcp.ParseString(request, "content", "")
		selector := mcp.ParseString(request, "selector", "")
		force := mcp.ParseBoolean(request, "forced_merge", false)
		if !strings.HasPrefix(path, "cue/") { return mcp.NewToolResultText("Denied"), nil }
		newContent, err := GetMergedContent(path, selector, content, force)
		if err != nil { return mcp.NewToolResultText(fmt.Sprintf("Merge error: %v", err)), nil }
		orig, _ := os.ReadFile(path)
		os.WriteFile(path, newContent, 0644)
		dir := filepath.Dir(path)
		cmd := exec.Command("cue", "vet", "./" + dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			os.WriteFile(path, orig, 0644)
			return mcp.NewToolResultText(fmt.Sprintf("Syntax validation FAILED:\n%s", string(out))), nil
		}
		if _, _, _, _, _, _, _, _, err := compiler.RunPipeline("."); err != nil {
			os.WriteFile(path, orig, 0644)
			return mcp.NewToolResultText(fmt.Sprintf("Architecture validation FAILED: %v", err)), nil
		}
		return mcp.NewToolResultText("Intent merged and validated successfully."), nil
	})

	s.AddTool(mcp.NewTool("run_preset",
		mcp.WithDescription("Run build, unit, lint"),
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
		return mcp.NewToolResultText(fmt.Sprintf("Preset %s finished: %s.", name, status)), nil
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}