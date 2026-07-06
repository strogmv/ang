package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func registerMigrationTools(addTool toolAdder) {
	// ang openapi — generate OpenAPI spec without full build
	addTool("ang_openapi", mcp.NewTool("ang_openapi",
		mcp.WithDescription("Generate OpenAPI 3.x spec from CUE intent without running a full build. "+
			"Useful in CI, for frontend teams, or to inspect the API contract quickly. "+
			"Returns the YAML content of the generated spec."),
		mcp.WithString("path", mcp.Description("Project root (default: .).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath := mcp.ParseString(request, "path", ".")
		args := []string{"openapi", "--stdout"}
		if strings.TrimSpace(projectPath) != "." {
			args = append(args, "--path", projectPath)
		}
		cmd := exec.Command(resolveANGExecutable(), args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("ang_openapi failed: %v\n%s", err, truncate(string(out), 4000))), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})

	// ang sdk version — read current version
	addTool("ang_sdk_version", mcp.NewTool("ang_sdk_version",
		mcp.WithDescription("Show the current project version from cue/project/project.cue."),
		mcp.WithString("path", mcp.Description("Project root (default: .).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath := mcp.ParseString(request, "path", ".")
		args := []string{"sdk", "version", "--path", projectPath}
		cmd := exec.Command(resolveANGExecutable(), args...)
		out, err := cmd.CombinedOutput()
		result := map[string]any{"version": strings.TrimSpace(string(out))}
		if err != nil {
			result["error"] = err.Error()
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	// ang sdk bump — bump version
	addTool("ang_sdk_bump", mcp.NewTool("ang_sdk_bump",
		mcp.WithDescription("Bump the project semver in cue/project/project.cue. "+
			"Use before releasing a new SDK or API version. "+
			"Returns old and new version."),
		mcp.WithString("part", mcp.Required(), mcp.Description("Which part to bump: patch | minor | major")),
		mcp.WithString("path", mcp.Description("Project root (default: .).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		part := mcp.ParseString(request, "part", "patch")
		projectPath := mcp.ParseString(request, "path", ".")
		switch part {
		case "patch", "minor", "major":
		default:
			return mcp.NewToolResultText(`{"error":"part must be patch|minor|major"}`), nil
		}
		args := []string{"sdk", part, "--path", projectPath}
		cmd := exec.Command(resolveANGExecutable(), args...)
		out, err := cmd.CombinedOutput()
		result := map[string]any{"output": strings.TrimSpace(string(out))}
		if err != nil {
			result["status"] = "failed"
			result["error"] = err.Error()
		} else {
			result["status"] = "ok"
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	// ang extract — extract facts from existing code
	addTool("ang_extract", mcp.NewTool("ang_extract",
		mcp.WithDescription("Extract deterministic facts (ang/facts/v1 JSON) from existing Go/Java/OpenAPI/SQL source. "+
			"First step of the AI migration pipeline. "+
			"Returns the facts envelope as JSON."),
		mcp.WithString("source_path", mcp.Required(), mcp.Description("Directory or file to analyze.")),
		mcp.WithString("from", mcp.Description("Source type: go | java | openapi | sql | auto (default: auto).")),
		mcp.WithString("out", mcp.Description("Write facts to this file (optional; if omitted, returns inline).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sourcePath := mcp.ParseString(request, "source_path", ".")
		from := mcp.ParseString(request, "from", "auto")
		outFile := mcp.ParseString(request, "out", "")

		args := []string{"extract", sourcePath, "--from", from}
		if strings.TrimSpace(outFile) != "" {
			args = append(args, "--out", outFile)
		}

		cmd := exec.Command(resolveANGExecutable(), args...)
		out, err := cmd.CombinedOutput()
		payload := bytes.TrimSpace(out)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf(`{"status":"failed","error":%q,"output":%q}`,
				err.Error(), truncate(string(payload), 4000))), nil
		}
		if json.Valid(payload) {
			return mcp.NewToolResultText(string(payload)), nil
		}
		return mcp.NewToolResultText(string(payload)), nil
	})

	// ang gen — AI-powered CUE generation from facts
	addTool("ang_gen", mcp.NewTool("ang_gen",
		mcp.WithDescription("Generate CUE operations from ang/facts/v1 JSON using Claude AI. "+
			"Requires ANTHROPIC_API_KEY environment variable. "+
			"Writes .cue files to cue/api/ (or --out). "+
			"Use --dry_run to preview without writing. "+
			"Full pipeline: ang_extract → ang_gen → ang_ops_vet → ang_generate."),
		mcp.WithString("facts_path", mcp.Required(), mcp.Description("Path to ang/facts/v1 JSON from ang_extract.")),
		mcp.WithString("service", mcp.Description("Generate only operations for this service hint (empty = all).")),
		mcp.WithString("out", mcp.Description("Output directory for CUE files (default: cue/api).")),
		mcp.WithBoolean("dry_run", mcp.Description("Print generated CUE without writing files.")),
		mcp.WithString("model", mcp.Description("Claude model ID (default: claude-opus-4-6).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		factsPath := mcp.ParseString(request, "facts_path", "")
		service := mcp.ParseString(request, "service", "")
		outDir := mcp.ParseString(request, "out", resolveMCPCueRoot(".")+"/api")
		dryRun := mcp.ParseBoolean(request, "dry_run", false)
		model := mcp.ParseString(request, "model", "")

		args := []string{"gen", "--facts", factsPath, "--out", outDir}
		if strings.TrimSpace(service) != "" {
			args = append(args, "--service", service)
		}
		if dryRun {
			args = append(args, "--dry-run")
		}
		if strings.TrimSpace(model) != "" {
			args = append(args, "--model", model)
		}

		cmd := exec.Command(resolveANGExecutable(), args...)
		out, err := cmd.CombinedOutput()
		result := map[string]any{
			"output": truncate(string(out), 8000),
		}
		if err != nil {
			result["status"] = "failed"
			result["error"] = err.Error()
			result["next_actions"] = []string{
				"Check ANTHROPIC_API_KEY is set",
				"Verify facts file path with ang_extract",
				"Try --dry_run to see what would be generated",
			}
		} else {
			result["status"] = "ok"
			result["next_actions"] = []string{
				"Run ang_ops_vet to validate generated CUE",
				"Run ang_ops_vet with fix=true to auto-fix issues",
				"Run ang_generate to compile into Go/Python code",
			}
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}
