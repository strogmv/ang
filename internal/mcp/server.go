package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
)

func getLock(path string) *sync.Mutex {
	lock, _ := projectLocks.LoadOrStore(path, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func validateCuePath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil { return err }
	cwd, _ := os.Getwd()
	cueDir := filepath.Join(cwd, "cue")
	if !strings.HasPrefix(abs, cueDir) {
		return fmt.Errorf("access denied: only files in /cue/ directory are modifiable")
	}
	if filepath.Ext(path) != ".cue" {
		return fmt.Errorf("access denied: only .cue files can be modified")
	}
	return nil
}

func validateReadPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil { return err }
	cwd, _ := os.Getwd()
	if !strings.HasPrefix(abs, cwd) {
		return fmt.Errorf("access denied: path %s is outside of workspace", path)
	}
	return nil
}

func Run() {
	s := server.NewMCPServer(
		"ANG MCP Server",
		compiler.Version,
		server.WithLogging(),
	)

	// --- CORE TOOLS ---

	s.AddTool(mcp.NewTool("ang_capabilities",
		mcp.WithDescription("Get ANG compiler capabilities and AI policy"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res := map[string]interface{}{
			"ang_version":    compiler.Version,
			"schema_version": compiler.SchemaVersion,
			"policy":         "Agent writes only CUE. ANG writes code. Agent reads code and runs tests.",
			"presets":        []string{"unit", "e2e", "lint", "build", "migrate"},
			"capabilities": []string{"ai_hints", "structured_diagnostics", "contract_coverage", "safe_apply", "planning"},
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	s.AddTool(mcp.NewTool("ang_plan",
		mcp.WithDescription("Propose a development plan for a given goal"),
		mcp.WithString("goal", mcp.Description("What do you want to achieve?"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		goal := mcp.ParseString(request, "goal", "")
		// Simple deterministic planner for common tasks
		plan := []string{"1. Call ang_validate to check current state"}
		if strings.Contains(strings.ToLower(goal), "entity") || strings.Contains(strings.ToLower(goal), "field") {
			plan = append(plan, "2. Modify CUE in cue/domain/ via cue_apply_patch")
			plan = append(plan, "3. Run run_preset('build') to generate implementation")
			plan = append(plan, "4. Verify with repo_diff and run_preset('unit')")
		} else {
			plan = append(plan, "2. Identify target CUE files", "3. Apply changes", "4. Rebuild and test")
		}
		
		res := map[string]interface{}{
			"goal": goal,
			"steps": plan,
			"suggested_tools": []string{"ang_validate", "cue_apply_patch", "ang_generate", "run_preset"},
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// --- CUE TOOLS (RW) ---

	s.AddTool(mcp.NewTool("cue_read",
		mcp.WithDescription("Read a CUE file from /cue directory"),
		mcp.WithString("path", mcp.Description("Path to .cue file"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "path", "")
		if err := validateReadPath(path); err != nil { return mcp.NewToolResultText(err.Error()), nil }
		content, err := os.ReadFile(path)
		if err != nil { return mcp.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil }
		return mcp.NewToolResultText(string(content)), nil
	})

	s.AddTool(mcp.NewTool("cue_apply_patch",
		mcp.WithDescription("Update a CUE file. ONLY /cue directory allowed."),
		mcp.WithString("path", mcp.Description("Path to .cue file"), mcp.Required()),
		mcp.WithString("content", mcp.Description("Full new content"), mcp.Required()),
		mcp.WithBoolean("dry_run", mcp.Description("Validate only")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, content := mcp.ParseString(request, "path", ""), mcp.ParseString(request, "content", "")
		dryRun := mcp.ParseBoolean(request, "dry_run", true)
		if err := validateCuePath(path); err != nil { return mcp.NewToolResultText(err.Error()), nil }
		if dryRun { return mcp.NewToolResultText("Validated (Dry-run)"), nil }
		if err := os.WriteFile(path, []byte(content), 0644); err != nil { return mcp.NewToolResultText(err.Error()), nil }
		return mcp.NewToolResultText("Updated successfully"), nil
	})

	// --- GENERATION & PRESETS ---

	s.AddTool(mcp.NewTool("run_preset",
		mcp.WithDescription("Run a predefined workflow command"),
		mcp.WithString("name", mcp.Description("Preset name: unit, e2e, lint, build, migrate"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")
		var cmd *exec.Cmd
		switch name {
		case "unit": cmd = exec.Command("go", "test", "-v", "./...")
		case "lint": cmd = exec.Command("./ang_bin", "lint")
		case "build": cmd = exec.Command("./ang_bin", "build")
		case "migrate": cmd = exec.Command("./ang_bin", "migrate", "diff", "auto")
		default: return mcp.NewToolResultText("Unknown preset: "+name), nil
		}
		
		out, err := cmd.CombinedOutput()
		res := map[string]interface{}{ "preset": name, "success": err == nil, "output": string(out) }
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	s.AddTool(mcp.NewTool("repo_read_symbol",
		mcp.WithDescription("Read a specific Go function or type definition"),
		mcp.WithString("symbol", mcp.Description("Symbol name, e.g. UserRepo.FindByID"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol := mcp.ParseString(request, "symbol", "")
		// Simple grep-based discovery for now
		cmd := exec.Command("grep", "-r", "-n", "-E", "func.*"+symbol+"|type.*"+symbol, "internal/")
		out, _ := cmd.CombinedOutput()
		return mcp.NewToolResultText(string(out)), nil
	})

	// --- RESOURCES ---

	registerResources(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func registerResources(s *server.MCPServer) {
	// AI Contract
	s.AddResource(mcp.NewResource("resource://ang/ai_contract", "AI Agent Contract", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			contract := map[string]interface{}{
				"version": compiler.Version,
				"workflow": []string{"validate", "patch", "build", "diff", "test"},
				"constraints": []string{"Agent writes only CUE", "ReadOnly access to code"},
				"budget_hints": map[string]string{"repo_diff": "preferred source for changes", "ir": "preferred source for schema"},
			}
			data, _ := json.MarshalIndent(contract, "", "  ")
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/ai_contract", MIMEType: "application/json", Text: string(data)}}, nil
		})

	// IR and others...
	s.AddResource(mcp.NewResource("resource://ang/ir", "Full IR", mcp.WithMIMEType("application/json")), 
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return readIRPart(request.Params.URI, func(schema *ir.Schema) interface{} { return schema })
		})
	
	s.AddResource(mcp.NewResource("resource://ang/ai_hints", "AI Agent Hints", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			hints := map[string]interface{}{"patterns": "Use logic.Call with list args", "anti_patterns": "Don't edit internal/*.go"}
			data, _ := json.MarshalIndent(hints, "", "  ")
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/ai_hints", MIMEType: "application/json", Text: string(data)}}, nil
		})
}

func readIRPart(uri string, selector func(*ir.Schema) interface{}) ([]mcp.ResourceContents, error) {
	entities, services, endpoints, repos, events, bizErrors, schedules, err := compiler.RunPipeline(".")
	if err != nil { return nil, err }
	schema := ir.ConvertFromNormalizer(entities, services, events, bizErrors, endpoints, repos, normalizer.ConfigDef{}, nil, nil, schedules, nil, normalizer.ProjectDef{})
	part := selector(schema)
	jsonRes, _ := json.MarshalIndent(part, "", "  ")
	return []mcp.ResourceContents{mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonRes)}}, nil
}