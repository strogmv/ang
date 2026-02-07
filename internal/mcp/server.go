package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

var (
	projectLocks sync.Map // map[string]*sync.Mutex
)

func getLock(path string) *sync.Mutex {
	lock, _ := projectLocks.LoadOrStore(path, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// ANGReport - Unified response structure
type ANGReport struct {
	ReportID    string               `json:"report_id"`
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

	// 1. Impact Analysis (Graph-based)
	s.AddTool(mcp.NewTool("ang_check_impact",
		mcp.WithDescription("Analyze architectural impact using dependency graph"),
		mcp.WithString("cue_path", mcp.Description("CUE path, e.g. cue://#User"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		target := mcp.ParseString(request, "cue_path", "")
		
		entities, services, endpoints, repos, events, bizErrors, schedules, err := compiler.RunPipeline(".")
		if err != nil { return mcp.NewToolResultText(err.Error()), nil }
		schema := ir.ConvertFromNormalizer(entities, services, events, bizErrors, endpoints, repos, normalizer.ConfigDef{}, nil, nil, schedules, nil, normalizer.ProjectDef{})

		impacts := []string{}
		for _, edge := range schema.Graph.Edges {
			if edge.To == target {
				impacts = append(impacts, fmt.Sprintf("%s (%s %s)", edge.From, edge.Type, edge.To))
			}
		}

		report := &ANGReport{
			ReportID: "imp_" + compiler.Version,
			Summary:  []string{fmt.Sprintf("Dependency analysis for: %s", target)},
			Impacts:  impacts,
		}
		if len(impacts) == 0 {
			report.Summary = append(report.Summary, "No direct dependencies found in current graph.")
		}

		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// 2. Verify Change (Dry-run Build)
	s.AddTool(mcp.NewTool("ang_verify_change",
		mcp.WithDescription("Run a full generation cycle in dry-run mode to verify CUE patches"),
		mcp.WithString("path", mcp.Description("CUE file to verify"), mcp.Required()),
		mcp.WithString("content", mcp.Description("Proposed new content"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, content := mcp.ParseString(request, "path", ""), mcp.ParseString(request, "content", "")
		
		// 1. Backup current file
		orig, _ := os.ReadFile(path)
		defer os.WriteFile(path, orig, 0644) // Restore after test

		// 2. Apply patch temporarily
		os.WriteFile(path, []byte(content), 0644)

		// 3. Run validate & build
		cmd := exec.Command("./ang_bin", "build")
		out, err := cmd.CombinedOutput()

		report := &ANGReport{
			ReportID: "verify_" + compiler.Version,
			Summary:  []string{"Dry-run verification finished"},
			Artifacts: map[string]string{"build_log": string(out)},
		}
		if err != nil {
			report.Summary = append(report.Summary, "Status: FAILED")
		} else {
			report.Summary = append(report.Summary, "Status: SUCCESS")
		}

		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// 3. Flow Query (Semantic)
	s.AddTool(mcp.NewTool("ang_flow_query",
		mcp.WithDescription("Find flow steps that interact with a specific entity"),
		mcp.WithString("entity", mcp.Description("Entity name, e.g. User"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entity := mcp.ParseString(request, "entity", "")
		
		_, services, _, _, _, _, _, _ := compiler.RunPipeline(".")
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

		report := &ANGReport{
			ReportID: "flow_" + compiler.Version,
			Summary:  []string{fmt.Sprintf("Flow interactions with %s", entity)},
			Impacts:  found,
		}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	// Capabilities
	s.AddTool(mcp.NewTool("ang_capabilities",
		mcp.WithDescription("Get ANG compiler capabilities"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res := map[string]interface{}{
			"ang_version": compiler.Version,
			"capabilities": []string{"impact_analysis_v2", "verify_change", "semantic_flow_query"},
			"report_format": "ANGReport v1",
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
