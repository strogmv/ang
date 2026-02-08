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
	"go/parser"
	"go/token"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
	parserpkg "github.com/strogmv/ang/compiler/parser"
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

	// --- LOGIC VALIDATOR (Stage 47) ---

	s.AddTool(mcp.NewTool("ang_validate_logic",
		mcp.WithDescription("Validate Go code snippet syntax before inserting into CUE"),
		mcp.WithString("code", mcp.Description("Go code block"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		code := mcp.ParseString(request, "code", "")
		
		// Wrap to make parsable
		wrapped := fmt.Sprintf("package dummy\nfunc _() { \n// Variables injected by ANG context\nvar req, resp, ctx, s, err interface{}\n_ = req; _ = resp; _ = ctx; _ = s; _ = err;\n%s\n}", code)
		
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, "", wrapped, 0)
		
		report := &ANGReport{
			Status: "Valid",
			Summary: []string{"Go logic validation"},
		}
		
		if err != nil {
			report.Status = "Invalid"
			report.Diagnostics = append(report.Diagnostics, normalizer.Warning{
				Kind: "logic", Code: "GO_SYNTAX_ERROR", Message: err.Error(), Severity: "error",
			})
			report.Rationale = "Syntax error detected in embedded Go code."
		} else {
			report.Summary = append(report.Summary, "Syntax is correct.")
		}

		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- RBAC INSPECTOR (Stage 45) ---

	s.AddTool(mcp.NewTool("ang_rbac_inspector",
		mcp.WithDescription("Audit RBAC actions and policies. Identifies valid action names and security holes."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, services, _, _, _, _, _, _, err := compiler.RunPipeline(".")
		if err != nil { return mcp.NewToolResultText(err.Error()), nil }
		
		validActions := make(map[string]bool)
		for _, s := range services {
			for _, m := range s.Methods {
				validActions[strings.ToLower(s.Name)+"."+strings.ToLower(m.Name)] = true
			}
		}

		p := parserpkg.New()
		n := normalizer.New()
		var rbac *normalizer.RBACDef
		if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/policies"); err == nil && ok {
			rbac, _ = n.ExtractRBAC(val)
		} else if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/rbac"); err == nil && ok {
			rbac, _ = n.ExtractRBAC(val)
		}

		unprotected := []string{}
		zombies := []string{}
		protected := []string{}

		if rbac != nil {
			for action := range rbac.Permissions {
				if validActions[action] {
					protected = append(protected, action)
				} else {
					zombies = append(zombies, action)
				}
			}
		}

		for action := range validActions {
			isFound := false
			for _, p := range protected { if p == action { isFound = true; break } }
			if !isFound { unprotected = append(unprotected, action) }
		}

		report := &ANGReport{
			Status: "Audited",
			Summary: []string{
				fmt.Sprintf("Protected: %d", len(protected)),
				fmt.Sprintf("Unprotected (Holes): %d", len(unprotected)),
				fmt.Sprintf("Zombies (Errors): %d", len(zombies)),
			},
			Impacts: unprotected,
			Artifacts: map[string]string{
				"valid_actions": strings.Join(protected, "\n"),
				"holes":         strings.Join(unprotected, "\n"),
				"zombies":       strings.Join(zombies, "\n"),
			},
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- DB SYNC (Stage 41) ---

	s.AddTool(mcp.NewTool("ang_db_sync",
		mcp.WithDescription("Synchronize database schema with current CUE intent (requires DATABASE_URL)"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("./ang_bin", "db", "sync")
		out, err := cmd.CombinedOutput()
		status := "Success"
		if err != nil { status = "Failed" }
		report := &ANGReport{ Status: status, Summary: []string{"Database synchronization results"}, Artifacts: map[string]string{"log": string(out)} }
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
				"feature_add": []string{"ang_plan", "ang_search", "ang_rbac_inspector", "ang_validate_logic", "cue_apply_patch", "run_preset('build')", "ang_db_sync"},
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