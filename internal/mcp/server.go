package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func validatePath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	if !strings.HasPrefix(abs, cwd) {
		return fmt.Errorf("access denied: path %s is outside of workspace", path)
	}
	return nil
}

func Run() {
	s := server.NewMCPServer(
		"ANG MCP Server",
		"0.1.0",
		server.WithLogging(),
	)

	// Tool: ang_validate
	s.AddTool(mcp.NewTool("ang_validate",
		mcp.WithDescription("Validate ANG project structure and CUE definitions with structured violations"),
		mcp.WithString("project_path", mcp.Description("Path to project root"), mcp.DefaultString(".")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "project_path", ".")
		if err := validatePath(path); err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}

		_, _, _, _, _, _, _, err := compiler.RunPipeline(path)
		
		res := map[string]interface{}{
			"valid":      err == nil && len(compiler.LatestDiagnostics) == 0,
			"violations": compiler.LatestDiagnostics,
		}
		if err != nil {
			res["error"] = err.Error()
		}

		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// Tool: ang_apply (Experimental patch tool)
	s.AddTool(mcp.NewTool("ang_apply",
		mcp.WithDescription("Apply structural changes or patches to CUE files"),
		mcp.WithString("file", mcp.Description("File to modify"), mcp.Required()),
		mcp.WithString("op", mcp.Description("Operation: replace, insert, delete"), mcp.Required()),
		mcp.WithString("text", mcp.Description("Text to use for operation")),
		mcp.WithNumber("line", mcp.Description("Line number")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		file := mcp.ParseString(request, "file", "")
		op := mcp.ParseString(request, "op", "")
		
		if err := validatePath(file); err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}

		// Very basic implementation for POC
		return mcp.NewToolResultText(fmt.Sprintf("Applied %s to %s (Simulated)", op, file)), nil
	})

	// Tool: ang_build
	s.AddTool(mcp.NewTool("ang_build",
		mcp.WithDescription("Build the project, generating Go code. Returns summary and changed files."),
		mcp.WithString("project_path", mcp.Description("Path to project root"), mcp.DefaultString(".")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "project_path", ".")
		if err := validatePath(path); err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}

		executable, err := os.Executable()
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Failed to find executable: %v", err)), nil
		}

		cmd := exec.Command(executable, "build")
		cmd.Dir = path
		output, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Build failed:\n%s\nError: %v", string(output), err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Build successful:\n%s", string(output))), nil
	})

	// Tool: ang_graph
	s.AddTool(mcp.NewTool("ang_graph",
		mcp.WithDescription("Generate architecture graph data"),
		mcp.WithString("project_path", mcp.Description("Path to project root"), mcp.DefaultString(".")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "project_path", ".")
		if err := validatePath(path); err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}

		entities, services, endpoints, _, _, _, _, err := compiler.RunPipeline(path)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Error analyzing project: %v", err)), nil
		}

		em := emitter.New(path, "sdk", "templates")
		mCtx := em.AnalyzeContext(services, entities, endpoints)
		
		jsonRes, _ := json.MarshalIndent(mCtx, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// Resource: Manifest
	s.AddResource(mcp.NewResource("resource://ang/manifest", "Manifest",
		mcp.WithResourceDescription("Current system manifest (ang-manifest.json)"),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		content, err := os.ReadFile("ang-manifest.json")
		if err != nil {
			return nil, fmt.Errorf("manifest not found (run ang build first): %w", err)
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "resource://ang/manifest",
				MIMEType: "application/json",
				Text:     string(content),
			},
		}, nil
	})

	// Resource: IR
	s.AddResource(mcp.NewResource("resource://ang/ir", "Intermediate Representation",
		mcp.WithResourceDescription("Full system IR dump"),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		entities, services, endpoints, repos, events, bizErrors, schedules, err := compiler.RunPipeline(".")
		if err != nil {
			return nil, err
		}
		
		// Convert to IR
		schema := ir.ConvertFromNormalizer(entities, services, events, bizErrors, endpoints, repos, normalizer.ConfigDef{}, nil, nil, schedules, nil, normalizer.ProjectDef{})
		
		jsonRes, _ := json.MarshalIndent(schema, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "resource://ang/ir",
				MIMEType: "application/json",
				Text:     string(jsonRes),
			},
		}, nil
	})

	// Resource: Diagnostics
	s.AddResource(mcp.NewResource("resource://ang/diagnostics/latest", "Latest Diagnostics",
		mcp.WithResourceDescription("Latest validation errors and warnings"),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		jsonRes, _ := json.MarshalIndent(compiler.LatestDiagnostics, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "resource://ang/diagnostics/latest",
				MIMEType: "application/json",
				Text:     string(jsonRes),
			},
		}, nil
	})

	// Resource: Project Summary
	s.AddResource(mcp.NewResource("resource://ang/project/summary", "Project Summary",
		mcp.WithResourceDescription("Short summary of the project targets and versions"),
		mcp.WithMIMEType("text/plain"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		entities, services, endpoints, _, _, _, _, _ := compiler.RunPipeline(".")
		summary := fmt.Sprintf("ANG Project Summary\nVersion: 0.1.0\nEntities: %d\nServices: %d\nEndpoints: %d\n", len(entities), len(services), len(endpoints))
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "resource://ang/project/summary",
				MIMEType: "text/plain",
				Text:     summary,
			},
		}, nil
	})

	// Prompt: add-entity
	s.AddPrompt(mcp.NewPrompt("add-entity",
		mcp.WithPromptDescription("Generate CUE snippet for a new entity and return ops for ang_apply"),
		mcp.WithArgument("name", mcp.ArgumentDescription("Name of the entity"), mcp.RequiredArgument()),
		mcp.WithArgument("fields", mcp.ArgumentDescription("Comma separated fields, e.g. title:string, price:int")),
	), func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		name := fmt.Sprintf("%v", request.Params.Arguments["name"])
		fieldsRaw := fmt.Sprintf("%v", request.Params.Arguments["fields"])
		
		var fieldsBuilder strings.Builder
		for _, f := range strings.Split(fieldsRaw, ",") {
			parts := strings.Split(strings.TrimSpace(f), ":")
			if len(parts) == 2 {
				fieldsBuilder.WriteString(fmt.Sprintf("\t%s: %s\n", parts[0], parts[1]))
			}
		}

		nameUpper := strings.ToUpper(name[:1]) + name[1:]
		snippet := fmt.Sprintf("%s: schema.#Entity & {\n%s}", nameUpper, fieldsBuilder.String())
		fileName := fmt.Sprintf("cue/domain/%s.cue", strings.ToLower(name))

		res := map[string]interface{}{
			"message": fmt.Sprintf("I will create a new entity '%s' in '%s'.", nameUpper, fileName),
			"ops": []map[string]interface{}{
				{
					"file": fileName,
					"op":   "create",
					"text": snippet,
				},
			},
		}

		jsonRes, _ := json.MarshalIndent(res, "", "  ")

		return mcp.NewGetPromptResult("CUE snippet for entity", []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(string(jsonRes))),
		}), nil
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}