package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

func goldenPatternSources() []string {
	data, err := os.ReadFile(filepath.Join("cue", "GOLDEN_EXAMPLES.cue"))
	if err != nil {
		return []string{"template:crud_entity", "template:fsm_entity", "template:event_service", "template:webhook_handler"}
	}
	lines := strings.Split(string(data), "\n")
	patterns := []string{}
	re := regexp.MustCompile(`^//\s*EXAMPLE\s+\d+:\s*(.+?)\s*$`)
	for _, ln := range lines {
		m := re.FindStringSubmatch(strings.TrimSpace(ln))
		if len(m) != 2 {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(m[1]))
		label = strings.ReplaceAll(label, " ", "_")
		label = strings.ReplaceAll(label, "/", "_")
		patterns = append(patterns, "golden:"+label)
	}
	if len(patterns) == 0 {
		return []string{"template:crud_entity", "template:fsm_entity", "template:event_service", "template:webhook_handler"}
	}
	return patterns
}

func buildGoalPlan(goal string) (map[string]any, error) {
	if isMarketplaceGoal(goal) {
		return buildMarketplacePlan(goal)
	}

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

	plan := map[string]any{
		"entities":             entityPlan,
		"services":             servicePlan,
		"endpoints":            endpointPlan,
		"gaps":                 gaps,
		"delta":                map[string]any{"add_entities": []string{}, "add_services": []string{}, "add_endpoints": []map[string]any{}},
		"cue_apply_patch":      []map[string]any{},
		"pattern_sources":      goldenPatternSources(),
		"estimated_iterations": estimate,
	}
	return map[string]any{"status": "planned", "goal": goal, "plan": plan}, nil
}

func isMarketplaceGoal(goal string) bool {
	g := strings.ToLower(strings.TrimSpace(goal))
	if g == "" {
		return false
	}
	return strings.Contains(g, "marketplace") ||
		(strings.Contains(g, "order") && strings.Contains(g, "payment") && strings.Contains(g, "notification"))
}

func buildMarketplacePlan(goal string) (map[string]any, error) {
	entities, services, endpoints, _, _, _, _, _, err := compiler.RunPipeline(".")
	if err != nil {
		return nil, err
	}

	entitySet := map[string]struct{}{}
	for _, e := range entities {
		entitySet[strings.ToLower(e.Name)] = struct{}{}
	}
	serviceSet := map[string]struct{}{}
	for _, s := range services {
		serviceSet[strings.ToLower(s.Name)] = struct{}{}
	}
	endpointSet := map[string]struct{}{}
	for _, ep := range endpoints {
		key := strings.ToUpper(ep.Method) + " " + ep.Path
		endpointSet[key] = struct{}{}
	}

	planEntities := []map[string]any{
		{"name": "Product", "fields": []string{"id", "title", "price", "categoryID", "sellerID"}, "file": "cue/domain/product.cue"},
		{"name": "Category", "fields": []string{"id", "name", "slug"}, "file": "cue/domain/product.cue"},
		{"name": "Cart", "fields": []string{"id", "buyerID", "status", "createdAt"}, "file": "cue/domain/product.cue"},
		{
			"name":   "Order",
			"fields": []string{"id", "buyerID", "status", "total"},
			"fsm": map[string]any{
				"field":  "status",
				"states": []string{"draft", "paid", "shipped", "delivered"},
			},
			"file": "cue/domain/order.cue",
		},
	}
	planServices := []map[string]any{
		{"name": "Orders", "methods": []string{"CreateOrder", "ConfirmPayment", "ShipOrder"}, "publishes": []string{"OrderPaid", "OrderShipped"}},
		{"name": "Notifications", "subscribes": map[string]string{"OrderPaid": "NotifySeller"}},
	}
	planEndpoints := []map[string]any{
		{"method": "POST", "path": "/orders", "rpc": "CreateOrder", "auth": "jwt"},
		{"method": "POST", "path": "/webhooks/stripe", "rpc": "ConfirmPayment", "auth": "none"},
	}

	addEntities := []string{}
	for _, e := range []string{"Product", "Category", "Cart", "Order"} {
		if _, ok := entitySet[strings.ToLower(e)]; !ok {
			addEntities = append(addEntities, e)
		}
	}
	addServices := []string{}
	for _, s := range []string{"Orders", "Notifications"} {
		if _, ok := serviceSet[strings.ToLower(s)]; !ok {
			addServices = append(addServices, s)
		}
	}
	addEndpoints := []map[string]any{}
	for _, ep := range planEndpoints {
		key := fmt.Sprintf("%s %s", ep["method"], ep["path"])
		if _, ok := endpointSet[key]; !ok {
			addEndpoints = append(addEndpoints, ep)
		}
	}

	productCue := `package domain

#Product: {
	name: "Product"
	fields: {
		id: {type: "uuid"}
		title: {type: "string"}
		price: {type: "int"}
		categoryID: {type: "uuid"}
		sellerID: {type: "uuid"}
	}
}

#Category: {
	name: "Category"
	fields: {
		id: {type: "uuid"}
		name: {type: "string"}
		slug: {type: "string"}
	}
}

#Cart: {
	name: "Cart"
	fields: {
		id: {type: "uuid"}
		buyerID: {type: "uuid"}
		status: {type: "string"}
		createdAt: {type: "time"}
	}
}
`

	orderCue := `package domain

#Order: {
	name: "Order"
	fields: {
		id: {type: "uuid"}
		buyerID: {type: "uuid"}
		status: {type: "string"}
		total: {type: "int"}
	}
	fsm: {
		field: "status"
		states: ["draft", "paid", "shipped", "delivered"]
		transitions: [
			{from: "draft", to: "paid"},
			{from: "paid", to: "shipped"},
			{from: "shipped", to: "delivered"},
		]
	}
}
`

	marketplaceCue := `package api

CreateOrder: {
	service: "orders"
	input: {
		buyerID: string
	}
	output: {
		ok: bool
	}
}

ConfirmPayment: {
	service: "orders"
	input: {
		orderID: string
		providerEventID: string
	}
	output: {
		ok: bool
	}
}

ShipOrder: {
	service: "orders"
	input: {
		orderID: string
	}
	output: {
		ok: bool
	}
}

HTTP: {
	CreateOrder: {
		method: "POST"
		path:   "/orders"
		auth:   "jwt"
	}
	ConfirmPayment: {
		method: "POST"
		path:   "/webhooks/stripe"
		auth:   "none"
	}
}
`

	servicesCue := `package architecture

#Services: {
	orders: {
		name: "Orders"
		entities: ["Order", "Cart"]
		publishes: ["OrderPaid", "OrderShipped"]
	}
	notifications: {
		name: "Notifications"
		subscribes: {
			OrderPaid: "NotifySeller"
		}
	}
}
`

	patches := []map[string]any{
		{"path": "cue/domain/product.cue", "selector": "", "forced_merge": true, "content": productCue},
		{"path": "cue/domain/order.cue", "selector": "", "forced_merge": true, "content": orderCue},
		{"path": "cue/api/marketplace.cue", "selector": "", "forced_merge": true, "content": marketplaceCue},
		{"path": "cue/architecture/services.cue", "selector": "", "forced_merge": true, "content": servicesCue},
	}

	plan := map[string]any{
		"entities":             planEntities,
		"services":             planServices,
		"endpoints":            planEndpoints,
		"delta":                map[string]any{"add_entities": addEntities, "add_services": addServices, "add_endpoints": addEndpoints},
		"cue_apply_patch":      patches,
		"pattern_sources":      goldenPatternSources(),
		"estimated_iterations": 2,
	}
	return map[string]any{"status": "planned", "goal": goal, "plan": plan}, nil
}

type toolAdder func(name string, tool mcp.Tool, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error))

func Run() {
	s := server.NewMCPServer(
		"ANG MCP Server",
		compiler.Version,
		server.WithPromptCapabilities(true),
		server.WithResourceCapabilities(false, true),
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
		_ = json.Unmarshal([]byte(`[    "ang_plan",    "ang_schema",    "ang_validate",    "ang_snapshot",    "ang_search",    "repo_read_symbol",    "ang_rbac_inspector",    "ang_event_map",    "ang_db_drift_detector",    "cue_set_field",    "cue_add_endpoint",    "cue_apply_patch",    "cue_history",    "run_preset('build')",    "ang_model_diff",    "ang_db_sync"]`), &base)
		if ov := loadRuntimeOverrides(); ov != nil {
			if wf, ok := ov.Workflows["feature_add"]; ok && len(wf) > 0 {
				return wf
			}
		}
		return base
	}

	bugFixWorkflow := func() []string {
		base := []string{}
		_ = json.Unmarshal([]byte(`[    "ang_schema",    "ang_validate",    "ang_snapshot",    "run_preset('unit')",    "ang_explain_error",    "ang_doctor",    "cue_set_field",    "cue_add_endpoint",    "cue_apply_patch",    "cue_history",    "cue_undo",    "run_preset('build')",    "ang_model_diff"]`), &base)
		if ov := loadRuntimeOverrides(); ov != nil {
			if wf, ok := ov.Workflows["bug_fix"]; ok && len(wf) > 0 {
				return wf
			}
		}
		return base
	}

	bootstrapExempt := func() map[string]bool {
		list := []string{}
		_ = json.Unmarshal([]byte(`[    "ang_mcp_health",    "ang_status",    "ang_schema",    "ang_dry_run"]`), &list)
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

	s.AddResource(mcp.NewResource(
		"resource://ang/readme_for_agents",
		"ANG Agent README",
		mcp.WithResourceDescription("Guidance for AI agents working in this repository."),
		mcp.WithMIMEType("text/plain"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		candidates := []string{
			filepath.Join(projectRoot, "AGENTS.md"),
			filepath.Join(projectRoot, "CLAUDE.md"),
			filepath.Join(projectRoot, "README.md"),
		}
		var data []byte
		for _, p := range candidates {
			b, err := os.ReadFile(p)
			if err == nil {
				data = b
				break
			}
		}
		if len(data) == 0 {
			data = []byte("No AGENTS.md/CLAUDE.md/README.md found in project root.")
		}
		return []mcp.ResourceContents{mcp.TextResourceContents{
			URI:      "resource://ang/readme_for_agents",
			MIMEType: "text/plain",
			Text:     string(data),
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
		s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (resp *mcp.CallToolResult, err error) {
			return safeInvokeTool(name, envelopeEnabled(), toolEnvelope, func() (*mcp.CallToolResult, error) {
				sessionState.Lock()
				bootstrapped := sessionState.Bootstrapped
				if !bootstrapExempt()[name] && !bootstrapped {
					sessionState.Bootstrapped = true
				}
				sessionState.Unlock()

				resp, err = h(ctx, request)
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
		})
	}

	toolCatalog := map[string]mcp.Tool{}
	addToolWithCatalog := func(name string, tool mcp.Tool, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
		toolCatalog[name] = tool
		addTool(name, tool, h)
	}
	registerDBTools(addToolWithCatalog)
	registerAnalysisTools(addToolWithCatalog)
	registerPlanTools(addToolWithCatalog)
	registerCoreTools(addToolWithCatalog, coreToolDeps{
		currentProfile:     currentProfile,
		runtimeConfigPath:  runtimeConfigPath,
		runtimeConfigError: runtimeConfigError,
		featureAddWorkflow: featureAddWorkflow,
		bugFixWorkflow:     bugFixWorkflow,
		bootstrapExempt:    bootstrapExempt,
		envelopeEnabled:    envelopeEnabled,
		searchLimits:       searchLimits,
		symbolLimits:       symbolLimits,
		snapshotLimits:     snapshotLimits,
		mcpSchemaVersion:   mcpSchemaVersion,
	})
	registerCUETools(addToolWithCatalog)
	registerListTools := func(name string) {
		addToolWithCatalog(name, mcp.NewTool(name,
			mcp.WithDescription("List all registered MCP tools with descriptions."),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			type toolItem struct {
				Name        string `json:"name"`
				Description string `json:"description,omitempty"`
			}
			tools := make([]toolItem, 0, len(toolCatalog))
			for name, def := range toolCatalog {
				tools = append(tools, toolItem{
					Name:        name,
					Description: strings.TrimSpace(def.Description),
				})
			}
			sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
			out := map[string]any{
				"count": len(tools),
				"tools": tools,
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		})
	}
	registerListTools("list_tools")
	registerListTools("ang_list_tools")
	registerPrompts(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}
}
