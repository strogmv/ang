package mcp

import (
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
}
