package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/strogmv/ang/compiler"
)

type coreToolDeps struct {
	currentProfile     func() string
	runtimeConfigPath  func() string
	runtimeConfigError func() string
	featureAddWorkflow func() []string
	bugFixWorkflow     func() []string
	bootstrapExempt    func() map[string]bool
	envelopeEnabled    func() bool
	searchLimits       func() (int, int)
	symbolLimits       func() (int, int)
	snapshotLimits     func() (int, int)
	mcpSchemaVersion   string
}

func registerCoreTools(addTool toolAdder, deps coreToolDeps) {
	addTool("ang_bootstrap", mcp.NewTool("ang_bootstrap",
		mcp.WithDescription("Mandatory first step."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionState.Lock()
		sessionState.Bootstrapped = true
		sessionState.Unlock()

		res := map[string]any{
			"status":              "Ready",
			"ang_version":         compiler.Version,
			"active_profile":      deps.currentProfile(),
			"runtime_config_path": deps.runtimeConfigPath(),
			"workflows": map[string]any{
				"feature_add": deps.featureAddWorkflow(),
				"bug_fix":     deps.bugFixWorkflow(),
			},
			"resources": []string{"resource://ang/logs/build", "resource://ang/policy"},
		}
		if errMsg := deps.runtimeConfigError(); errMsg != "" {
			res["runtime_config_error"] = errMsg
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	addTool("ang_mcp_health", mcp.NewTool("ang_mcp_health",
		mcp.WithDescription("MCP health and effective limits/workflow diagnostics."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionState.Lock()
		bootstrapped := sessionState.Bootstrapped
		lastAction := sessionState.LastAction
		sessionState.Unlock()

		searchDefault, searchHardCap := deps.searchLimits()
		symbolDefault, symbolHardCap := deps.symbolLimits()
		snapshotDefault, snapshotHardCap := deps.snapshotLimits()

		exemptList := []string{}
		for name := range deps.bootstrapExempt() {
			exemptList = append(exemptList, name)
		}
		sort.Strings(exemptList)

		health := map[string]any{
			"status":              "ok",
			"ang_version":         compiler.Version,
			"schema_version":      deps.mcpSchemaVersion,
			"envelope_enabled":    deps.envelopeEnabled(),
			"active_profile":      deps.currentProfile(),
			"bootstrapped":        bootstrapped,
			"last_action":         lastAction,
			"runtime_config_path": deps.runtimeConfigPath(),
			"limits": map[string]any{
				"search":      map[string]int{"default": searchDefault, "hard_cap": searchHardCap},
				"symbol_read": map[string]int{"default": symbolDefault, "hard_cap": symbolHardCap},
				"snapshot":    map[string]int{"default": snapshotDefault, "hard_cap": snapshotHardCap},
			},
			"workflows": map[string]any{
				"feature_add": deps.featureAddWorkflow(),
				"bug_fix":     deps.bugFixWorkflow(),
			},
			"bootstrap_exempt_tools": exemptList,
		}
		if errMsg := deps.runtimeConfigError(); errMsg != "" {
			health["runtime_config_error"] = errMsg
			health["status"] = "warn"
		}
		b, _ := json.MarshalIndent(health, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_snapshot", mcp.NewTool("ang_snapshot",
		mcp.WithDescription("Compact project snapshot for low-token context handoff."),
		mcp.WithNumber("max_status_lines", mcp.Description("Max git status lines (profile-based default/cap).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		defaultLines, hardCap := deps.snapshotLimits()
		maxLines := int(mcp.ParseFloat64(request, "max_status_lines", float64(defaultLines)))
		if maxLines <= 0 {
			maxLines = defaultLines
		}
		if maxLines > hardCap {
			maxLines = hardCap
		}

		cmd := exec.Command("git", "status", "--short")
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) == 0 {
			return mcp.NewToolResultText(err.Error()), nil
		}
		statusLines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		if len(statusLines) == 1 && statusLines[0] == "" {
			statusLines = []string{}
		}
		totalStatus := len(statusLines)
		truncated := false
		if len(statusLines) > maxLines {
			statusLines = statusLines[:maxLines]
			truncated = true
		}

		report := map[string]any{
			"status":              "snapshot",
			"profile":             deps.currentProfile(),
			"last_action":         sessionState.LastAction,
			"git_status_total":    totalStatus,
			"git_status_returned": len(statusLines),
			"truncated":           truncated,
			"git_status_lines":    statusLines,
			"next_actions":        []string{"Edit CUE intent", "Run run_preset('build')", "Run targeted tests"},
			"token_hints":         []string{"Use repo_read_symbol for pinpoint reads", "Pass max_lines to ang_search"},
		}
		b, _ := json.MarshalIndent(report, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_schema", mcp.NewTool("ang_schema",
		mcp.WithDescription("Compact domain schema view: entities, services, endpoints."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entities, services, endpoints, _, _, _, _, _, err := compiler.RunPipeline(".")
		if err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}

		sort.Slice(entities, func(i, j int) bool {
			return strings.ToLower(entities[i].Name) < strings.ToLower(entities[j].Name)
		})
		sort.Slice(services, func(i, j int) bool {
			return strings.ToLower(services[i].Name) < strings.ToLower(services[j].Name)
		})
		sort.Slice(endpoints, func(i, j int) bool {
			li := strings.ToUpper(endpoints[i].Method) + " " + endpoints[i].Path + " " + endpoints[i].RPC
			lj := strings.ToUpper(endpoints[j].Method) + " " + endpoints[j].Path + " " + endpoints[j].RPC
			return li < lj
		})

		entityOut := make([]map[string]any, 0, len(entities))
		for _, e := range entities {
			fields := make([]string, 0, len(e.Fields))
			for _, f := range e.Fields {
				fields = append(fields, f.Name)
			}
			sort.Strings(fields)
			entityOut = append(entityOut, map[string]any{
				"name":   e.Name,
				"fields": fields,
			})
		}

		serviceOut := make([]map[string]any, 0, len(services))
		for _, s := range services {
			methods := make([]string, 0, len(s.Methods))
			for _, m := range s.Methods {
				methods = append(methods, m.Name)
			}
			sort.Strings(methods)
			serviceOut = append(serviceOut, map[string]any{
				"name":    s.Name,
				"methods": methods,
			})
		}

		endpointOut := make([]map[string]string, 0, len(endpoints))
		for _, ep := range endpoints {
			endpointOut = append(endpointOut, map[string]string{
				"method": strings.ToUpper(ep.Method),
				"path":   ep.Path,
				"rpc":    ep.RPC,
			})
		}

		res := map[string]any{
			"status":    "ok",
			"profile":   deps.currentProfile(),
			"entities":  entityOut,
			"services":  serviceOut,
			"endpoints": endpointOut,
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_search", mcp.NewTool("ang_search",
		mcp.WithDescription("Hybrid symbol search with capped output."),
		mcp.WithString("query", mcp.Required()),
		mcp.WithNumber("max_lines", mcp.Description("Max output lines (profile-based default/cap).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := strings.TrimSpace(mcp.ParseString(request, "query", ""))
		if query == "" {
			return mcp.NewToolResultText("query is required"), nil
		}
		defaultLines, hardCap := deps.searchLimits()
		maxLines := int(mcp.ParseFloat64(request, "max_lines", float64(defaultLines)))
		if maxLines <= 0 {
			maxLines = defaultLines
		}
		if maxLines > hardCap {
			maxLines = hardCap
		}

		cmd := exec.Command("rg", "-n", "-i", query, "cue/", "internal/")
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) == 0 {
			return mcp.NewToolResultText(err.Error()), nil
		}
		lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = []string{}
		}
		sort.Strings(lines)
		totalMatches := len(lines)
		truncated := false
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			truncated = true
		}
		report := map[string]any{
			"query":         query,
			"profile":       deps.currentProfile(),
			"total_matches": totalMatches,
			"returned":      len(lines),
			"truncated":     truncated,
			"results":       lines,
		}
		b, _ := json.MarshalIndent(report, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("repo_read_symbol", mcp.NewTool("repo_read_symbol",
		mcp.WithDescription("Read compact symbol snippet from a file to save context tokens."),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("symbol", mcp.Required()),
		mcp.WithNumber("context", mcp.Description("Context lines before/after (profile-based default/cap).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := strings.TrimSpace(mcp.ParseString(request, "path", ""))
		symbol := strings.TrimSpace(mcp.ParseString(request, "symbol", ""))
		defaultCtx, hardCap := deps.symbolLimits()
		ctxLines := int(mcp.ParseFloat64(request, "context", float64(defaultCtx)))
		if ctxLines < 0 {
			ctxLines = 0
		}
		if ctxLines > hardCap {
			ctxLines = hardCap
		}
		if path == "" || symbol == "" {
			return mcp.NewToolResultText("path and symbol are required"), nil
		}
		if err := validateReadPath(path); err != nil {
			return mcp.NewToolResultText("invalid path"), nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}
		lines := strings.Split(string(data), "\n")
		needleA := "func " + symbol + "("
		needleB := "type " + symbol + " "
		start := -1
		for i, ln := range lines {
			trim := strings.TrimSpace(ln)
			if strings.HasPrefix(trim, needleA) || strings.HasPrefix(trim, needleB) || strings.Contains(trim, " "+symbol+" struct") {
				start = i
				break
			}
		}
		if start == -1 {
			return mcp.NewToolResultText("symbol not found"), nil
		}
		from := start - ctxLines
		if from < 0 {
			from = 0
		}
		to := start + ctxLines + 1
		if to > len(lines) {
			to = len(lines)
		}
		var b bytes.Buffer
		for i := from; i < to; i++ {
			fmt.Fprintf(&b, "%d:%s\n", i+1, lines[i])
		}
		report := map[string]any{
			"path":      path,
			"symbol":    symbol,
			"profile":   deps.currentProfile(),
			"from":      from + 1,
			"to":        to,
			"truncated": (from > 0 || to < len(lines)),
			"snippet":   b.String(),
		}
		out, _ := json.MarshalIndent(report, "", "  ")
		return mcp.NewToolResultText(string(out)), nil
	})
}
