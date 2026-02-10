package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

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
		LastAction   string
		Bootstrapped bool
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

func sourceFile(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	if idx := strings.LastIndex(source, ":"); idx > 0 {
		return source[:idx]
	}
	return source
}

func detectErrorCodes(log string) []string {
	re := regexp.MustCompile(`\b(E_[A-Z0-9_]+|[A-Z]+_[A-Z0-9_]*_ERROR)\b`)
	matches := re.FindAllString(log, -1)
	if len(matches) == 0 {
		return nil
	}

	known := map[string]struct{}{
		"E_FSM_UNDEFINED_STATE": {},
	}
	for _, c := range compiler.StableErrorCodes {
		known[c] = struct{}{}
	}

	uniq := map[string]struct{}{}
	for _, m := range matches {
		if _, ok := known[m]; ok {
			uniq[m] = struct{}{}
		}
	}
	if len(uniq) == 0 {
		return nil
	}
	out := make([]string, 0, len(uniq))
	for c := range uniq {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func buildGoalPlan(goal string) (map[string]any, error) {
	entities, services, endpoints, _, _, _, _, _, err := compiler.RunPipeline(".")
	if err != nil {
		return nil, err
	}

	sort.Slice(entities, func(i, j int) bool { return entities[i].Name < entities[j].Name })
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	sort.Slice(endpoints, func(i, j int) bool {
		li := strings.ToUpper(endpoints[i].Method) + " " + endpoints[i].Path + " " + endpoints[i].RPC
		lj := strings.ToUpper(endpoints[j].Method) + " " + endpoints[j].Path + " " + endpoints[j].RPC
		return li < lj
	})

	entityPlan := make([]map[string]any, 0, len(entities))
	for _, e := range entities {
		fields := make([]string, 0, len(e.Fields))
		for _, f := range e.Fields {
			fields = append(fields, f.Name)
		}
		item := map[string]any{
			"name":   e.Name,
			"fields": fields,
			"file":   sourceFile(e.Source),
		}
		if e.FSM != nil {
			item["fsm"] = map[string]any{
				"field":       e.FSM.Field,
				"states":      append([]string(nil), e.FSM.States...),
				"transitions": e.FSM.Transitions,
			}
		}
		entityPlan = append(entityPlan, item)
	}

	servicePlan := make([]map[string]any, 0, len(services))
	for _, s := range services {
		methods := make([]string, 0, len(s.Methods))
		for _, m := range s.Methods {
			methods = append(methods, m.Name)
		}
		sort.Strings(methods)

		item := map[string]any{
			"name":       s.Name,
			"methods":    methods,
			"publishes":  append([]string(nil), s.Publishes...),
			"subscribes": s.Subscribes,
			"file":       sourceFile(s.Source),
		}
		servicePlan = append(servicePlan, item)
	}

	endpointPlan := make([]map[string]any, 0, len(endpoints))
	for _, ep := range endpoints {
		auth := "none"
		if strings.TrimSpace(ep.AuthType) != "" {
			auth = ep.AuthType
		}
		endpointPlan = append(endpointPlan, map[string]any{
			"method":  strings.ToUpper(ep.Method),
			"path":    ep.Path,
			"rpc":     ep.RPC,
			"service": ep.ServiceName,
			"auth":    auth,
			"file":    sourceFile(ep.Source),
		})
	}

	gaps := []string{}
	lowerGoal := strings.ToLower(strings.TrimSpace(goal))
	hasStripe := strings.Contains(lowerGoal, "stripe")
	hasWebhookGoal := strings.Contains(lowerGoal, "webhook")
	hasEmailGoal := strings.Contains(lowerGoal, "email") || strings.Contains(lowerGoal, "notification")
	hasOrderGoal := strings.Contains(lowerGoal, "order")
	if hasStripe || hasWebhookGoal {
		found := false
		for _, ep := range endpoints {
			p := strings.ToLower(ep.Path)
			if strings.Contains(p, "webhook") || strings.Contains(p, "stripe") {
				found = true
				break
			}
		}
		if !found {
			gaps = append(gaps, "No webhook endpoint found for payment provider integration.")
		}
	}
	if hasEmailGoal {
		found := false
		for _, s := range services {
			if strings.Contains(strings.ToLower(s.Name), "notif") {
				found = true
				break
			}
		}
		if !found {
			gaps = append(gaps, "No notification-focused service detected.")
		}
	}
	if hasOrderGoal {
		found := false
		for _, e := range entities {
			if strings.EqualFold(e.Name, "Order") {
				found = true
				break
			}
		}
		if !found {
			gaps = append(gaps, "Order entity is not present in current intent.")
		}
	}

	estimate := 2
	if len(gaps) > 0 {
		estimate = 3
	}

	return map[string]any{
		"status": "planned",
		"goal":   goal,
		"plan": map[string]any{
			"entities":  entityPlan,
			"services":  servicePlan,
			"endpoints": endpointPlan,
			"gaps":      gaps,
		},
		"estimated_iterations": estimate,
	}, nil
}

func Run() {
	s := server.NewMCPServer(
		"ANG MCP Server",
		compiler.Version,
		server.WithResourceCapabilities(false, true),
		server.WithLogging(),
	)

	defaultProfile := strings.ToLower(strings.TrimSpace("eco"))
	if defaultProfile == "" {
		defaultProfile = "eco"
	}
	runtimeConfigPathDefault := strings.TrimSpace(".ang/mcp-runtime.json")
	mcpSchemaVersion := strings.TrimSpace("mcp-envelope/v1")
	if mcpSchemaVersion == "" {
		mcpSchemaVersion = "mcp-envelope/v1"
	}

	type runtimeLimit struct {
		Default int `json:"default"`
		HardCap int `json:"hard_cap"`
	}
	type runtimeProfile struct {
		Search     runtimeLimit `json:"search"`
		SymbolRead runtimeLimit `json:"symbol_read"`
		Snapshot   runtimeLimit `json:"snapshot"`
	}
	type runtimeOverrides struct {
		DefaultProfile       string                    `json:"default_profile"`
		Profiles             map[string]runtimeProfile `json:"profiles"`
		Workflows            map[string][]string       `json:"workflows"`
		BootstrapExemptTools []string                  `json:"bootstrap_exempt_tools"`
	}

	loadRuntimeOverridesWithMeta := func() (*runtimeOverrides, string, string) {
		path := strings.TrimSpace(os.Getenv("ANG_MCP_RUNTIME_CONFIG"))
		if path == "" {
			path = runtimeConfigPathDefault
		}
		if path == "" {
			return nil, "", ""
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, path, ""
			}
			return nil, path, err.Error()
		}
		if len(data) == 0 {
			return nil, path, ""
		}
		var ov runtimeOverrides
		if err := json.Unmarshal(data, &ov); err != nil {
			return nil, path, "invalid runtime config JSON: " + err.Error()
		}
		return &ov, path, ""
	}
	loadRuntimeOverrides := func() *runtimeOverrides {
		ov, _, _ := loadRuntimeOverridesWithMeta()
		return ov
	}
	runtimeConfigPath := func() string {
		_, path, _ := loadRuntimeOverridesWithMeta()
		return path
	}
	runtimeConfigError := func() string {
		_, _, e := loadRuntimeOverridesWithMeta()
		return e
	}

	currentProfile := func() string {
		p := strings.ToLower(strings.TrimSpace(os.Getenv("ANG_MCP_PROFILE")))
		if p == "eco" || p == "full" {
			return p
		}
		if ov := loadRuntimeOverrides(); ov != nil {
			rp := strings.ToLower(strings.TrimSpace(ov.DefaultProfile))
			if rp == "eco" || rp == "full" {
				return rp
			}
		}
		return defaultProfile
	}

	searchLimits := func() (int, int) {
		p := currentProfile()
		var d, c int
		if p == "full" {
			d, c = 80, 300
		} else {
			d, c = 40, 120
		}
		if ov := loadRuntimeOverrides(); ov != nil {
			if prof, ok := ov.Profiles[p]; ok {
				if prof.Search.Default > 0 {
					d = prof.Search.Default
				}
				if prof.Search.HardCap > 0 {
					c = prof.Search.HardCap
				}
			}
		}
		return d, c
	}

	symbolLimits := func() (int, int) {
		p := currentProfile()
		var d, c int
		if p == "full" {
			d, c = 8, 50
		} else {
			d, c = 6, 24
		}
		if ov := loadRuntimeOverrides(); ov != nil {
			if prof, ok := ov.Profiles[p]; ok {
				if prof.SymbolRead.Default > 0 {
					d = prof.SymbolRead.Default
				}
				if prof.SymbolRead.HardCap > 0 {
					c = prof.SymbolRead.HardCap
				}
			}
		}
		return d, c
	}

	snapshotLimits := func() (int, int) {
		p := currentProfile()
		var d, c int
		if p == "full" {
			d, c = 40, 200
		} else {
			d, c = 25, 80
		}
		if ov := loadRuntimeOverrides(); ov != nil {
			if prof, ok := ov.Profiles[p]; ok {
				if prof.Snapshot.Default > 0 {
					d = prof.Snapshot.Default
				}
				if prof.Snapshot.HardCap > 0 {
					c = prof.Snapshot.HardCap
				}
			}
		}
		return d, c
	}

	featureAddWorkflow := func() []string {
		base := []string{}
		_ = json.Unmarshal([]byte(`[    "ang_plan",    "ang_snapshot",    "ang_search",    "repo_read_symbol",    "ang_rbac_inspector",    "ang_event_map",    "ang_db_drift_detector",    "cue_apply_patch",    "run_preset('build')",    "ang_db_sync"]`), &base)
		if ov := loadRuntimeOverrides(); ov != nil {
			if wf, ok := ov.Workflows["feature_add"]; ok && len(wf) > 0 {
				return wf
			}
		}
		return base
	}

	bugFixWorkflow := func() []string {
		base := []string{}
		_ = json.Unmarshal([]byte(`[    "ang_snapshot",    "run_preset('unit')",    "ang_explain_error",    "ang_doctor",    "cue_apply_patch",    "run_preset('build')"]`), &base)
		if ov := loadRuntimeOverrides(); ov != nil {
			if wf, ok := ov.Workflows["bug_fix"]; ok && len(wf) > 0 {
				return wf
			}
		}
		return base
	}

	bootstrapExempt := func() map[string]bool {
		list := []string{}
		_ = json.Unmarshal([]byte(`[    "ang_bootstrap",    "ang_mcp_health"]`), &list)
		if ov := loadRuntimeOverrides(); ov != nil && len(ov.BootstrapExemptTools) > 0 {
			list = ov.BootstrapExemptTools
		}
		m := map[string]bool{}
		for _, t := range list {
			m[t] = true
		}
		return m
	}

	projectRoot, _ := os.Getwd()
	cueRoot := filepath.Join(projectRoot, "cue")

	resolveCueURI := func(uri string) (string, error) {
		const prefix = "resource://ang/cue/"
		if !strings.HasPrefix(uri, prefix) {
			return "", fmt.Errorf("invalid cue resource uri")
		}
		rel := strings.TrimPrefix(uri, prefix)
		rel = strings.TrimPrefix(rel, "/")
		rel = filepath.Clean(rel)
		if rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
			return "", fmt.Errorf("invalid cue path")
		}
		full := filepath.Join(cueRoot, rel)
		back, err := filepath.Rel(cueRoot, full)
		if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(os.PathSeparator)) {
			return "", fmt.Errorf("path escapes cue root")
		}
		return full, nil
	}

	s.AddResource(mcp.NewResource(
		"resource://ang/logs/build",
		"ANG Build Log",
		mcp.WithResourceDescription("Build log from current project CWD (ang-build.log)."),
		mcp.WithMIMEType("text/plain"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		logPath := filepath.Join(projectRoot, "ang-build.log")
		b, err := os.ReadFile(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				b = []byte("ang-build.log not found in current project")
			} else {
				return nil, err
			}
		}
		return []mcp.ResourceContents{mcp.TextResourceContents{
			URI:      "resource://ang/logs/build",
			MIMEType: "text/plain",
			Text:     string(b),
		}}, nil
	})

	s.AddResource(mcp.NewResource(
		"resource://ang/policy",
		"ANG MCP Policy",
		mcp.WithResourceDescription("Current MCP policy/profile summary for this project session."),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		policy := map[string]any{
			"status":                 "ok",
			"schema_version":         mcpSchemaVersion,
			"active_profile":         currentProfile(),
			"bootstrap_exempt_tools": bootstrapExempt(),
			"workflows": map[string]any{
				"feature_add": featureAddWorkflow(),
				"bug_fix":     bugFixWorkflow(),
			},
		}
		out, _ := json.MarshalIndent(policy, "", "  ")
		return []mcp.ResourceContents{mcp.TextResourceContents{
			URI:      "resource://ang/policy",
			MIMEType: "application/json",
			Text:     string(out),
		}}, nil
	})

	s.AddResourceTemplate(mcp.NewResourceTemplate(
		"resource://ang/cue/{path}",
		"ANG CUE File",
		mcp.WithTemplateDescription("Read a concrete CUE file from the current project CWD/cue directory."),
		mcp.WithTemplateMIMEType("text/plain"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		full, err := resolveCueURI(request.Params.URI)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     string(b),
		}}, nil
	})

	envelopeEnabled := func() bool {
		v := strings.ToLower(strings.TrimSpace(os.Getenv("ANG_MCP_ENVELOPE")))
		switch v {
		case "", "1", "true", "on", "yes":
			return true
		case "0", "false", "off", "no":
			return false
		default:
			return true
		}
	}

	toolEnvelope := func(name string, status string, payload any, extra map[string]any) *mcp.CallToolResult {
		body := map[string]any{
			"tool":           name,
			"status":         status,
			"active_profile": currentProfile(),
			"schema_version": mcpSchemaVersion,
			"payload":        payload,
		}
		for k, v := range extra {
			body[k] = v
		}
		b, _ := json.MarshalIndent(body, "", "  ")
		return mcp.NewToolResultText(string(b))
	}

	normalizeToolResult := func(name string, resp *mcp.CallToolResult) *mcp.CallToolResult {
		if resp == nil {
			return toolEnvelope(name, "ok", map[string]any{}, nil)
		}
		if resp.StructuredContent != nil {
			return toolEnvelope(name, "ok", resp.StructuredContent, nil)
		}

		var text string
		for _, c := range resp.Content {
			tc, ok := c.(mcp.TextContent)
			if ok {
				text = tc.Text
				break
			}
		}
		if text == "" {
			st := "ok"
			if resp.IsError {
				st = "tool_error"
			}
			return toolEnvelope(name, st, map[string]any{"note": "non-text MCP content"}, nil)
		}

		var parsed any
		if json.Unmarshal([]byte(text), &parsed) == nil {
			st := "ok"
			if resp.IsError {
				st = "tool_error"
			}
			return toolEnvelope(name, st, parsed, nil)
		}

		st := "ok"
		if resp.IsError {
			st = "tool_error"
		}
		return toolEnvelope(name, st, map[string]any{"message": text}, nil)
	}

	addTool := func(name string, tool mcp.Tool, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
		s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			sessionState.Lock()
			bootstrapped := sessionState.Bootstrapped
			sessionState.Unlock()
			if !bootstrapExempt()[name] && !bootstrapped {
				if envelopeEnabled() {
					return toolEnvelope(name, "blocked", map[string]any{
						"reason": "call ang_bootstrap first",
					}, nil), nil
				}
				res, _ := json.MarshalIndent(map[string]any{
					"status": "blocked",
					"reason": "call ang_bootstrap first",
					"tool":   name,
				}, "", "  ")
				return mcp.NewToolResultText(string(res)), nil
			}

			resp, err := h(ctx, request)
			if err != nil {
				if envelopeEnabled() {
					return toolEnvelope(name, "tool_error", map[string]any{
						"message": err.Error(),
					}, nil), nil
				}
				return nil, err
			}

			sessionState.Lock()
			sessionState.LastAction = name
			sessionState.Unlock()
			if envelopeEnabled() {
				return normalizeToolResult(name, resp), nil
			}
			return resp, nil
		})
	}
	// --- DB DRIFT DETECTOR (Stage 49) ---

	addTool("ang_db_drift_detector", mcp.NewTool("ang_db_drift_detector",
		mcp.WithDescription("Detect discrepancies between CUE domain models and physical database schema"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("./ang_bin", "db", "status")
		out, err := cmd.CombinedOutput()

		report := &ANGReport{
			Status:  "In Sync",
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

	addTool("ang_event_map", mcp.NewTool("ang_event_map",
		mcp.WithDescription("Map event publishers and subscribers to visualize system-wide reactions"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, services, _, _, _, _, _, _, err := compiler.RunPipeline(".")
		if err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}
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
			Event      string   `json:"event"`
			ProducedBy []string `json:"produced_by"`
			ConsumedBy []string `json:"consumed_by"`
			IsDeadEnd  bool     `json:"is_dead_end"`
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
		return mcp.NewToolResultText((&ANGReport{Status: "Mapped", Artifacts: map[string]string{"event_flows": string(artifacts)}}).ToJSON()), nil
	})

	// --- LOGIC VALIDATOR ---

	addTool("ang_validate_logic", mcp.NewTool("ang_validate_logic",
		mcp.WithDescription("Validate Go code snippet syntax before inserting into CUE"),
		mcp.WithString("code", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		code := mcp.ParseString(request, "code", "")
		wrapped := fmt.Sprintf("package dummy\nfunc _() { \nvar req, resp, ctx, s, err interface{}\n_ = req; _ = resp; _ = ctx; _ = s; _ = err;\n%s\n}", code)
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, "", wrapped, 0)
		report := &ANGReport{Status: "Valid"}
		if err != nil {
			report.Status = "Invalid"
			report.Diagnostics = append(report.Diagnostics, normalizer.Warning{Kind: "logic", Code: "GO_SYNTAX_ERROR", Message: err.Error(), Severity: "error"})
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- RBAC INSPECTOR ---

	addTool("ang_rbac_inspector", mcp.NewTool("ang_rbac_inspector",
		mcp.WithDescription("Audit RBAC actions and policies."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, services, _, _, _, _, _, _, err := compiler.RunPipeline(".")
		if err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}
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
				if validActions[action] {
					protected = append(protected, action)
				}
			}
		}
		for action := range validActions {
			isFound := false
			for _, p := range protected {
				if p == action {
					isFound = true
					break
				}
			}
			if !isFound {
				unprotected = append(unprotected, action)
			}
		}
		report := &ANGReport{Status: "Audited", Impacts: unprotected}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- DB SYNC ---

	addTool("ang_db_sync", mcp.NewTool("ang_db_sync",
		mcp.WithDescription("Synchronize database schema with current CUE intent."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("./ang_bin", "db", "sync")
		out, err := cmd.CombinedOutput()
		status := "Success"
		if err != nil {
			status = "Failed"
		}
		report := &ANGReport{Status: status, Artifacts: map[string]string{"log": string(out)}}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- INTENT DEBUGGER ---

	addTool("ang_explain_error", mcp.NewTool("ang_explain_error",
		mcp.WithDescription("Map runtime error back to CUE intent."),
		mcp.WithString("log", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		log := mcp.ParseString(request, "log", "")
		re := regexp.MustCompile(`"intent":\s*"([^"]+):(\d+)(?:\s*\(([^)]+)\))?"`)
		matches := re.FindStringSubmatch(log)
		if len(matches) < 3 {
			return mcp.NewToolResultText("No intent found."), nil
		}
		file, line := matches[1], matches[2]
		content, _ := os.ReadFile(file)
		lines := strings.Split(string(content), "\n")
		snippet := ""
		lineIdx := 0
		fmt.Sscanf(line, "%d", &lineIdx)
		if lineIdx > 0 && lineIdx <= len(lines) {
			start := lineIdx - 3
			if start < 0 {
				start = 0
			}
			end := lineIdx + 2
			if end > len(lines) {
				end = len(lines)
			}
			snippet = strings.Join(lines[start:end], "\n")
		}
		report := &ANGReport{Status: "Debugging", Artifacts: map[string]string{"cue_snippet": snippet}}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- AI HEALER ---

	addTool("ang_plan", mcp.NewTool("ang_plan",
		mcp.WithDescription("Create a structured architecture plan from a natural-language goal and current CUE intent."),
		mcp.WithString("goal", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		goal := strings.TrimSpace(mcp.ParseString(request, "goal", ""))
		if goal == "" {
			return mcp.NewToolResultText(`{"status":"invalid","message":"goal is required"}`), nil
		}
		plan, err := buildGoalPlan(goal)
		if err != nil {
			return mcp.NewToolResultText((&ANGReport{
				Status:      "Failed",
				Summary:     []string{"Unable to build plan from current intent."},
				NextActions: []string{"Fix CUE validation errors and retry ang_plan"},
				Artifacts:   map[string]string{"error": err.Error()},
			}).ToJSON()), nil
		}
		b, _ := json.MarshalIndent(plan, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_doctor", mcp.NewTool("ang_doctor",
		mcp.WithDescription("Analyze build logs and suggest fixes."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logData, _ := os.ReadFile("ang-build.log")
		log := string(logData)
		report := &ANGReport{Status: "Analyzing", Summary: []string{"Scanning build logs for structured compiler codes."}}
		codes := detectErrorCodes(log)
		if len(codes) > 0 {
			report.Summary = append(report.Summary, fmt.Sprintf("Detected %d known error code(s).", len(codes)))
			report.Artifacts = map[string]string{
				"error_codes": strings.Join(codes, ","),
			}
		}
		if strings.Contains(log, "E_FSM_UNDEFINED_STATE") {
			missingState := "undefined"
			if m := regexp.MustCompile(`undefined state '([^']+)'`).FindStringSubmatch(log); len(m) == 2 {
				missingState = m[1]
			}
			report.Diagnostics = append(report.Diagnostics, normalizer.Warning{
				Kind:         "architecture",
				Code:         "E_FSM_UNDEFINED_STATE",
				Severity:     "error",
				Message:      "FSM transition references state not listed in fsm.states.",
				CanAutoApply: true,
				Hint:         fmt.Sprintf("Add '%s' to fsm.states or adjust transition edges.", missingState),
				SuggestedFix: []normalizer.Fix{{
					Kind:      "replace",
					CUEPath:   "cue/domain/*.cue:#*.fsm.states",
					Text:      fmt.Sprintf("append state '%s' to states", missingState),
					Rationale: "Keep FSM transitions closed over declared states.",
				}},
			})
		}
		if strings.Contains(log, "range can't iterate over") {
			report.Diagnostics = append(report.Diagnostics, normalizer.Warning{
				Kind: "template", Code: "LIST_REQUIRED", Message: "logic.Call args must be a list.", CanAutoApply: true,
			})
		}
		if len(report.Diagnostics) == 0 && len(codes) == 0 {
			report.Summary = append(report.Summary, "No known structured issues detected.")
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- MANDATORY ENTRYPOINT ---

	addTool("ang_bootstrap", mcp.NewTool("ang_bootstrap",
		mcp.WithDescription("Mandatory first step."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionState.Lock()
		sessionState.Bootstrapped = true
		sessionState.Unlock()

		res := map[string]interface{}{
			"status":              "Ready",
			"ang_version":         compiler.Version,
			"active_profile":      currentProfile(),
			"runtime_config_path": runtimeConfigPath(),
			"workflows": map[string]interface{}{
				"feature_add": featureAddWorkflow(),
				"bug_fix":     bugFixWorkflow(),
			},
			"resources": []string{"resource://ang/logs/build", "resource://ang/policy"},
		}
		if errMsg := runtimeConfigError(); errMsg != "" {
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

		searchDefault, searchHardCap := searchLimits()
		symbolDefault, symbolHardCap := symbolLimits()
		snapshotDefault, snapshotHardCap := snapshotLimits()

		exemptList := []string{}
		for name := range bootstrapExempt() {
			exemptList = append(exemptList, name)
		}
		sort.Strings(exemptList)

		health := map[string]any{
			"status":              "ok",
			"ang_version":         compiler.Version,
			"schema_version":      mcpSchemaVersion,
			"envelope_enabled":    envelopeEnabled(),
			"active_profile":      currentProfile(),
			"bootstrapped":        bootstrapped,
			"last_action":         lastAction,
			"runtime_config_path": runtimeConfigPath(),
			"limits": map[string]any{
				"search":      map[string]int{"default": searchDefault, "hard_cap": searchHardCap},
				"symbol_read": map[string]int{"default": symbolDefault, "hard_cap": symbolHardCap},
				"snapshot":    map[string]int{"default": snapshotDefault, "hard_cap": snapshotHardCap},
			},
			"workflows": map[string]any{
				"feature_add": featureAddWorkflow(),
				"bug_fix":     bugFixWorkflow(),
			},
			"bootstrap_exempt_tools": exemptList,
		}
		if errMsg := runtimeConfigError(); errMsg != "" {
			health["runtime_config_error"] = errMsg
			health["status"] = "warn"
		}
		b, _ := json.MarshalIndent(health, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	// --- ESSENTIAL TOOLS ---

	addTool("ang_snapshot", mcp.NewTool("ang_snapshot",
		mcp.WithDescription("Compact project snapshot for low-token context handoff."),
		mcp.WithNumber("max_status_lines", mcp.Description("Max git status lines (profile-based default/cap).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		defaultLines, hardCap := snapshotLimits()
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
			"profile":             currentProfile(),
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

	addTool("ang_search", mcp.NewTool("ang_search",
		mcp.WithDescription("Hybrid symbol search with capped output."),
		mcp.WithString("query", mcp.Required()),
		mcp.WithNumber("max_lines", mcp.Description("Max output lines (profile-based default/cap).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := strings.TrimSpace(mcp.ParseString(request, "query", ""))
		if query == "" {
			return mcp.NewToolResultText("query is required"), nil
		}
		defaultLines, hardCap := searchLimits()
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
			"profile":       currentProfile(),
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
		defaultCtx, hardCap := symbolLimits()
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
			"profile":   currentProfile(),
			"from":      from + 1,
			"to":        to,
			"truncated": (from > 0 || to < len(lines)),
			"snippet":   b.String(),
		}
		out, _ := json.MarshalIndent(report, "", "  ")
		return mcp.NewToolResultText(string(out)), nil
	})

	addTool("cue_apply_patch", mcp.NewTool("cue_apply_patch",
		mcp.WithDescription("Update CUE intent with atomic validation"),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("content", mcp.Required()),
		mcp.WithString("selector", mcp.Description("Target node path")),
		mcp.WithBoolean("forced_merge", mcp.Description("Overwrite instead of deep merge")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, content := mcp.ParseString(request, "path", ""), mcp.ParseString(request, "content", "")
		selector := mcp.ParseString(request, "selector", "")
		force := mcp.ParseBoolean(request, "forced_merge", false)
		if err := validateCuePath(path); err != nil {
			return mcp.NewToolResultText("Denied"), nil
		}
		newContent, err := GetMergedContent(path, selector, content, force)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Merge error: %v", err)), nil
		}
		orig, _ := os.ReadFile(path)
		os.WriteFile(path, newContent, 0644)
		dir := filepath.Dir(path)
		cmd := exec.Command("cue", "vet", "./"+dir)
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

	addTool("run_preset", mcp.NewTool("run_preset",
		mcp.WithDescription("Run build, unit, lint"),
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
		logFile := "ang-build.log"
		f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		cmd.Stdout, cmd.Stderr = f, f
		err := cmd.Run()
		f.Close()
		status := "SUCCESS"
		if err != nil {
			status = "FAILED"
		}
		return mcp.NewToolResultText(fmt.Sprintf("Preset %s finished: %s.", name, status)), nil
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
