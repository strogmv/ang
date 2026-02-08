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

	// --- DB DRIFT DETECTOR (Stage 49) ---

	s.AddTool(mcp.NewTool("ang_db_drift_detector",
		mcp.WithDescription("Detect discrepancies between CUE domain models and physical database schema"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("./ang_bin", "db", "status")
		out, err := cmd.CombinedOutput()
		
		report := &ANGReport{
			Status: "In Sync",
			Summary: []string{"Checking database drift"},
		}
		
		output := string(out)
		if err != nil || strings.Contains(output, "DRIFT DETECTED") || (len(output) > 0 && !strings.Contains(output, "in sync")) {
			report.Status = "Drift Detected"
			report.Summary = append(report.Summary, "Database schema is out of sync with CUE.")
			report.Artifacts = map[string]string{"sql_diff": output}
			report.NextActions = append(report.NextActions, "Run ang_db_sync to apply changes")
			report.Rationale = "Your CUE definitions changed, but the database still uses the old schema."
		} else {
			report.Summary = append(report.Summary, "Schema is healthy.")
		}

		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- EVENT MAPPER ---

	s.AddTool(mcp.NewTool("ang_event_map",
		mcp.WithDescription("Map event publishers and subscribers to visualize system-wide reactions"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, services, _, _, _, _, _, _, err := compiler.RunPipeline(".")
		if err != nil { return mcp.NewToolResultText(err.Error()), nil }
		publishers := make(map[string][]string)
		subscribers := make(map[string][]string)
		allEvents := make(map[string]bool)
		for _, s := range services {
			for _, m := range s.Methods {
				for _, p := range m.Publishes {
					publishers[p] = append(publishers[p], fmt.Sprintf("%s.%s", s.Name, m.Name))
					allEvents[p] = true
				}
			}
			for evt, handler := range s.Subscribes {
				subscribers[evt] = append(subscribers[evt], fmt.Sprintf("%s (Handler: %s)", s.Name, handler))
				allEvents[evt] = true
			}
		}
		type EventFlow struct {
			Event       string   `json:"event"`
			ProducedBy  []string `json:"produced_by"`
			ConsumedBy  []string `json:"consumed_by"`
			IsDeadEnd   bool     `json:"is_dead_end"`
		}
		var flows []EventFlow
		for evt := range allEvents {
			flows = append(flows, EventFlow{
				Event:      evt,
				ProducedBy: publishers[evt],
				ConsumedBy: subscribers[evt],
				IsDeadEnd:  len(subscribers[evt]) == 0,
			})
		}
		artifacts, _ := json.MarshalIndent(flows, "", "  ")
		return mcp.NewToolResultText((&ANGReport{ Status: "Mapped", Artifacts: map[string]string{"event_flows": string(artifacts)} }).ToJSON()), nil
	})

	// --- LOGIC VALIDATOR ---

	s.AddTool(mcp.NewTool("ang_validate_logic",
		mcp.WithDescription("Validate Go code snippet syntax before inserting into CUE"),
		mcp.WithString("code", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		code := mcp.ParseString(request, "code", "")
		wrapped := fmt.Sprintf("package dummy\nfunc _() { \nvar req, resp, ctx, s, err interface{}\n_ = req; _ = resp; _ = ctx; _ = s; _ = err;\n%s\n}", code)
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, "", wrapped, 0)
		report := &ANGReport{ Status: "Valid" }
		if err != nil {
			report.Status = "Invalid"
			report.Diagnostics = append(report.Diagnostics, normalizer.Warning{ Kind: "logic", Code: "GO_SYNTAX_ERROR", Message: err.Error(), Severity: "error" })
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- RBAC INSPECTOR ---

	s.AddTool(mcp.NewTool("ang_rbac_inspector",
		mcp.WithDescription("Audit RBAC actions and policies."),
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
		protected := []string{}
		if rbac != nil {
			for action := range rbac.Permissions {
				if validActions[action] { protected = append(protected, action) }
			}
		}
		for action := range validActions {
			isFound := false
			for _, p := range protected { if p == action { isFound = true; break } }
			if !isFound { unprotected = append(unprotected, action) }
		}
		report := &ANGReport{ Status: "Audited", Impacts: unprotected }
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- DB SYNC ---

	s.AddTool(mcp.NewTool("ang_db_sync",
		mcp.WithDescription("Synchronize database schema with current CUE intent."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("./ang_bin", "db", "sync")
		out, err := cmd.CombinedOutput()
		status := "Success"
		if err != nil { status = "Failed" }
		report := &ANGReport{ Status: status, Artifacts: map[string]string{"log": string(out)} }
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- INTENT DEBUGGER ---

	s.AddTool(mcp.NewTool("ang_explain_error",
		mcp.WithDescription("Map runtime error back to CUE intent."),
		mcp.WithString("log", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		log := mcp.ParseString(request, "log", "")
		re := regexp.MustCompile(`"intent":\s*"([^"]+):(\d+)(?:\s*\(([^)]+)\))?"`)
		matches := re.FindStringSubmatch(log)
		if len(matches) < 3 { return mcp.NewToolResultText("No intent found."), nil }
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
		report := &ANGReport{ Status: "Debugging", Artifacts: map[string]string{"cue_snippet": snippet} }
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- AI HEALER ---

	s.AddTool(mcp.NewTool("ang_doctor",
		mcp.WithDescription("Analyze build logs and suggest fixes."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logData, _ := os.ReadFile("ang-build.log")
		log := string(logData)
		report := &ANGReport{ Status: "Analyzing" }
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
				"feature_add": []string{"ang_plan", "ang_search", "ang_rbac_inspector", "ang_event_map", "ang_db_drift_detector", "cue_apply_patch", "run_preset('build')", "ang_db_sync"},
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