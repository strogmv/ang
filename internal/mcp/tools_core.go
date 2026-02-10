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
	"github.com/strogmv/ang/compiler/normalizer"
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

	addTool("ang_status", mcp.NewTool("ang_status",
		mcp.WithDescription("Project status in one call: build, tests, warnings, dirty files."),
		mcp.WithBoolean("run_tests", mcp.Description("Run go test ./... for fresh unit status (default: false).")),
		mcp.WithBoolean("run_go_build", mcp.Description("Run go build ./... to detect module-level compile errors (default: false).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		runTests := mcp.ParseBoolean(request, "run_tests", false)
		runGoBuild := mcp.ParseBoolean(request, "run_go_build", false)

		buildStatus := map[string]any{"status": "ok"}
		if _, _, _, _, _, _, _, _, err := compiler.RunPipeline("."); err != nil {
			buildStatus["status"] = "failed"
			buildStatus["error"] = err.Error()
		}

		testStatus := map[string]any{"status": "skipped"}
		if runTests {
			testStatus = runCommandStatus([]string{"go", "test", "./..."})
		}
		goBuildStatus := map[string]any{"status": "skipped"}
		if runGoBuild {
			goBuildStatus = runCommandStatus([]string{"go", "build", "./..."})
		}

		dirtyFiles := gitDirtyFiles(200)
		logWarnings := collectBuildWarnings("ang-build.log", 30)

		overall := "ok"
		if buildStatus["status"] == "failed" {
			overall = "failed"
		} else if s, _ := testStatus["status"].(string); s == "failed" {
			overall = "failed"
		} else if s, _ := goBuildStatus["status"].(string); s == "failed" {
			overall = "failed"
		} else if len(logWarnings) > 0 {
			overall = "warn"
		}

		report := map[string]any{
			"status":            overall,
			"profile":           deps.currentProfile(),
			"build":             buildStatus,
			"tests":             testStatus,
			"go_build":          goBuildStatus,
			"dirty_files":       dirtyFiles,
			"dirty_files_count": len(dirtyFiles),
			"warnings":          logWarnings,
		}
		b, _ := json.MarshalIndent(report, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_validate", mcp.NewTool("ang_validate",
		mcp.WithDescription("Fast validation without full code generation: pipeline + diagnostics."),
		mcp.WithString("project_path", mcp.Description("Project root path (default: current directory).")),
		mcp.WithBoolean("run_go_build", mcp.Description("Run go build ./... after pipeline validation (default: false).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath := strings.TrimSpace(mcp.ParseString(request, "project_path", ""))
		if projectPath == "" {
			projectPath = "."
		}
		runGoBuild := mcp.ParseBoolean(request, "run_go_build", false)
		_, _, _, _, _, _, _, _, err := compiler.RunPipeline(projectPath)
		diags := append([]any(nil), warningsToAny(compiler.LatestDiagnostics)...)
		status := "ok"
		if err != nil {
			status = "failed"
		}
		goBuildStatus := map[string]any{"status": "skipped"}
		if runGoBuild {
			goBuildStatus = runCommandStatus([]string{"go", "build", "./..."})
			if s, _ := goBuildStatus["status"].(string); s == "failed" {
				status = "failed"
			}
		}
		resp := map[string]any{
			"status":            status,
			"project_path":      projectPath,
			"error":             "",
			"diagnostics_count": len(diags),
			"diagnostics":       diags,
			"go_build":          goBuildStatus,
		}
		if err != nil {
			resp["error"] = err.Error()
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_dry_run", mcp.NewTool("ang_dry_run",
		mcp.WithDescription("Run `ang build --dry-run` and return preview output for AI-safe planning."),
		mcp.WithString("project_path", mcp.Description("Project root path (default: current directory).")),
		mcp.WithString("target", mcp.Description("Optional target name, same as --target.")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath := strings.TrimSpace(mcp.ParseString(request, "project_path", ""))
		target := strings.TrimSpace(mcp.ParseString(request, "target", ""))
		args := []string{"build"}
		if projectPath != "" {
			args = append(args, projectPath)
		}
		args = append(args, "--dry-run", "--log-format=json")
		if target != "" {
			args = append(args, "--target="+target)
		}
		cmd := exec.Command(resolveANGExecutable(), args...)
		out, err := cmd.CombinedOutput()
		text := string(out)
		status := "success"
		if err != nil || strings.Contains(text, "Build FAILED") {
			status = "failed"
		}
		resp := map[string]any{
			"status":         status,
			"command":        strings.Join(append([]string{resolveANGExecutable()}, args...), " "),
			"project_path":   projectPath,
			"target":         target,
			"output_excerpt": truncate(text, 10000),
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
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
		mcp.WithString("entity", mcp.Description("Optional entity-name filter.")),
		mcp.WithString("service", mcp.Description("Optional service-name filter.")),
		mcp.WithString("endpoint", mcp.Description("Optional endpoint filter by method/path/rpc substring.")),
		mcp.WithBoolean("include_fields", mcp.Description("Include entity fields (default true).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entityFilter := strings.TrimSpace(mcp.ParseString(request, "entity", ""))
		serviceFilter := strings.TrimSpace(mcp.ParseString(request, "service", ""))
		endpointFilter := strings.TrimSpace(mcp.ParseString(request, "endpoint", ""))
		includeFields := mcp.ParseBoolean(request, "include_fields", true)

		snap, err := buildModelSnapshot(".", includeFields)
		if err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}
		snap = filterModelSnapshot(snap, entityFilter, serviceFilter, endpointFilter)

		res := map[string]any{
			"status":          "ok",
			"profile":         deps.currentProfile(),
			"filters":         map[string]any{"entity": entityFilter, "service": serviceFilter, "endpoint": endpointFilter, "include_fields": includeFields},
			"entities":        snap.Entities,
			"services":        snap.Services,
			"endpoints":       snap.Endpoints,
			"entities_count":  len(snap.Entities),
			"services_count":  len(snap.Services),
			"endpoints_count": len(snap.Endpoints),
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_model_diff", mcp.NewTool("ang_model_diff",
		mcp.WithDescription("Diff domain model (entities/services/endpoints) against git ref."),
		mcp.WithString("base_ref", mcp.Description("Git ref to compare against (default HEAD).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		baseRef := strings.TrimSpace(mcp.ParseString(request, "base_ref", "HEAD"))
		if baseRef == "" {
			baseRef = "HEAD"
		}
		baseSnap, err := buildModelSnapshotFromGitRef(baseRef, true)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("ang_model_diff base snapshot failed: %v", err)), nil
		}
		currSnap, err := buildModelSnapshot(".", true)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("ang_model_diff current snapshot failed: %v", err)), nil
		}
		diff := diffModelSnapshots(baseSnap, currSnap)
		resp := map[string]any{
			"status":   "ok",
			"base_ref": baseRef,
			"diff":     diff,
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_search", mcp.NewTool("ang_search",
		mcp.WithDescription("Hybrid symbol search with capped output."),
		mcp.WithString("query", mcp.Required()),
		mcp.WithString("scope", mcp.Description("Search scope: cue | generated | templates | all (default).")),
		mcp.WithNumber("max_lines", mcp.Description("Max output lines (profile-based default/cap).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := strings.TrimSpace(mcp.ParseString(request, "query", ""))
		if query == "" {
			return mcp.NewToolResultText("query is required"), nil
		}
		scope := strings.ToLower(strings.TrimSpace(mcp.ParseString(request, "scope", "all")))
		searchRoots, err := searchScopeRoots(scope)
		if err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}
		defaultLines, hardCap := deps.searchLimits()
		maxLines := int(mcp.ParseFloat64(request, "max_lines", float64(defaultLines)))
		if maxLines <= 0 {
			maxLines = defaultLines
		}
		if maxLines > hardCap {
			maxLines = hardCap
		}

		rgArgs := []string{"-n", "-i", query}
		rgArgs = append(rgArgs, searchRoots...)
		cmd := exec.Command("rg", rgArgs...)
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
			"scope":         scope,
			"roots":         searchRoots,
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

func searchScopeRoots(scope string) ([]string, error) {
	switch scope {
	case "", "all":
		return []string{"cue/", "internal/", "templates/", "sdk/", "api/"}, nil
	case "cue":
		return []string{"cue/"}, nil
	case "generated":
		return []string{"internal/", "sdk/", "api/"}, nil
	case "templates":
		return []string{"templates/"}, nil
	default:
		return nil, fmt.Errorf("invalid scope %q; allowed: cue | generated | templates | all", scope)
	}
}

func runCommandStatus(cmdAndArgs []string) map[string]any {
	if len(cmdAndArgs) == 0 {
		return map[string]any{"status": "failed", "error": "empty command"}
	}
	cmd := exec.Command(cmdAndArgs[0], cmdAndArgs[1:]...)
	out, err := cmd.CombinedOutput()
	res := map[string]any{
		"command": strings.Join(cmdAndArgs, " "),
		"output":  tailLines(string(out), 30),
	}
	if err != nil {
		res["status"] = "failed"
		res["error"] = err.Error()
		return res
	}
	res["status"] = "ok"
	return res
}

func gitDirtyFiles(max int) []string {
	if max <= 0 {
		max = 200
	}
	cmd := exec.Command("git", "status", "--short")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}
	}
	if len(lines) > max {
		lines = lines[:max]
	}
	sort.Strings(lines)
	return lines
}

func collectBuildWarnings(logPath string, max int) []string {
	if max <= 0 {
		max = 30
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	out := []string{}
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		low := strings.ToLower(l)
		if strings.Contains(low, "warn") || strings.Contains(low, "warning") {
			out = append(out, l)
		}
	}
	if len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}

func warningsToAny(diags []normalizer.Warning) []any {
	out := make([]any, 0, len(diags))
	for _, d := range diags {
		out = append(out, map[string]any{
			"kind":           d.Kind,
			"code":           d.Code,
			"severity":       d.Severity,
			"message":        d.Message,
			"op":             d.Op,
			"step":           d.Step,
			"action":         d.Action,
			"file":           d.File,
			"line":           d.Line,
			"column":         d.Column,
			"cue_path":       d.CUEPath,
			"hint":           d.Hint,
			"docs_url":       d.DocsURL,
			"can_auto_apply": d.CanAutoApply,
		})
	}
	return out
}
