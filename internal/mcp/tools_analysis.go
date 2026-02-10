package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
	parserpkg "github.com/strogmv/ang/compiler/parser"
)

func registerAnalysisTools(addTool toolAdder) {
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
		type eventFlow struct {
			Event      string   `json:"event"`
			ProducedBy []string `json:"produced_by"`
			ConsumedBy []string `json:"consumed_by"`
			IsDeadEnd  bool     `json:"is_dead_end"`
		}
		var flows []eventFlow
		for evt := range allEvents {
			flows = append(flows, eventFlow{
				Event:      evt,
				ProducedBy: publishers[evt],
				ConsumedBy: subscribers[evt],
				IsDeadEnd:  len(subscribers[evt]) == 0,
			})
		}
		artifacts, _ := json.MarshalIndent(flows, "", "  ")
		return mcp.NewToolResultText((&ANGReport{Status: "Mapped", Artifacts: map[string]string{"event_flows": string(artifacts)}}).ToJSON()), nil
	})

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
}
