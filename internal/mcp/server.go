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
	"github.com/strogmv/ang/compiler/ir"
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

// ANGReport - Unified response structure for AI efficiency
type ANGReport struct {
	Status      string               `json:"status"`
	Summary     []string             `json:"summary"`
	Diagnostics []normalizer.Warning `json:"diagnostics,omitempty"`
	Impacts     []string             `json:"impacts,omitempty"`
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
				"feature_add": []string{"ang_plan", "ang_search", "cue_apply_patch", "ang_verify_change", "run_preset('build')", "run_preset('unit')"},
				"bug_fix":     []string{"run_preset('unit')", "ang_flow_query", "cue_apply_patch", "run_preset('build')"},
			},
			"resources": []string{
				"resource://ang/logs/build",
				"resource://ang/coverage/contract",
				"resource://ang/ir",
				"resource://ang/ai_hints",
			},
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// --- IMPACT & NAVIGATION ---

	s.AddTool(mcp.NewTool("ang_check_impact",
		mcp.WithDescription("Analyze architectural impact using dependency graph"),
		mcp.WithString("target", mcp.Description("CUE path, e.g. cue://#User"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		target := mcp.ParseString(request, "target", "")
		entities, services, endpoints, repos, events, bizErrors, schedules, scenarios, err := compiler.RunPipeline(".")
		if err != nil { return mcp.NewToolResultText(err.Error()), nil }
		schema := ir.ConvertFromNormalizer(entities, services, events, bizErrors, endpoints, repos, normalizer.ConfigDef{}, nil, nil, schedules, nil, normalizer.ProjectDef{})
		_ = scenarios

		impacts := []string{}
		if schema.Graph != nil {
			for _, edge := range schema.Graph.Edges {
				if edge.To == target {
					impacts = append(impacts, fmt.Sprintf("%s (%s %s)", edge.From, edge.Type, edge.To))
				}
			}
		}

		report := &ANGReport{
			Status: "Analyzed",
			Summary: []string{fmt.Sprintf("Dependency analysis for: %s", target)},
			Impacts: impacts,
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	s.AddTool(mcp.NewTool("ang_flow_query",
		mcp.WithDescription("Find flow steps that interact with a specific entity"),
		mcp.WithString("entity", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entity := mcp.ParseString(request, "entity", "")
		_, services, _, _, _, _, _, _, _ := compiler.RunPipeline(".")
		var found []string
		for _, svc := range services {
			for _, m := range svc.Methods {
				for _, step := range m.Flow {
					for _, val := range step.Args {
						if fmt.Sprint(val) == entity {
							found = append(found, fmt.Sprintf("%s.%s step: %s", svc.Name, m.Name, step.Action))
						}
					}
				}
			}
		}
		report := &ANGReport{ Status: "Searched", Impacts: found }
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// --- ESSENTIAL TOOLS ---

	s.AddTool(mcp.NewTool("ang_search",
		mcp.WithDescription("Symbol search across code and CUE intent"),
		mcp.WithString("query", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := mcp.ParseString(request, "query", "")
		cmd := exec.Command("grep", "-r", "-n", "-i", query, "cue/", "internal/")
		out, _ := cmd.CombinedOutput()
		return mcp.NewToolResultText(string(out)), nil
	})

	s.AddTool(mcp.NewTool("ang_verify_change",
		mcp.WithDescription("Dry-run build verification"),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("content", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, content := mcp.ParseString(request, "path", ""), mcp.ParseString(request, "content", "")
		orig, _ := os.ReadFile(path)
		defer os.WriteFile(path, orig, 0644)
		os.WriteFile(path, []byte(content), 0644)
		cmd := exec.Command("./ang_bin", "build")
		out, err := cmd.CombinedOutput()
		status := "SUCCESS"
		if err != nil { status = "FAILED" }
		report := &ANGReport{ Status: status, Artifacts: map[string]string{"build_log": string(out)} }
		return mcp.NewToolResultText(report.ToJSON()), nil
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

	s.AddTool(mcp.NewTool("cue_apply_patch",
		mcp.WithDescription("Update CUE intent"),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("content", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, content := mcp.ParseString(request, "path", ""), mcp.ParseString(request, "content", "")
		if !strings.HasPrefix(path, "cue/") { return mcp.NewToolResultText("Denied"), nil }
		os.WriteFile(path, []byte(content), 0644)
		return mcp.NewToolResultText("Updated. Next: run_preset('build')"), nil
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

	s.AddResource(mcp.NewResource("resource://ang/coverage/contract", "Contract Coverage", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			ent, svcs, _, _, _, _, _, _, _ := compiler.RunPipeline(".")
			res := map[string]interface{}{ "entities": len(ent), "services": len(svcs) }
			data, _ := json.MarshalIndent(res, "", "  ")
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/coverage/contract", MIMEType: "application/json", Text: string(data)}}, nil
		})
}