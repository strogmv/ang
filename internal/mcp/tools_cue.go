package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/strogmv/ang/compiler"
)

func registerCUETools(addTool toolAdder) {
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
		os.WriteFile(path, newContent, 0o644)
		dir := filepath.Dir(path)
		cmd := exec.Command("cue", "vet", "./"+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			os.WriteFile(path, orig, 0o644)
			return mcp.NewToolResultText(fmt.Sprintf("Syntax validation FAILED:\n%s", string(out))), nil
		}
		if _, _, _, _, _, _, _, _, err := compiler.RunPipeline("."); err != nil {
			os.WriteFile(path, orig, 0o644)
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
		f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		cmd.Stdout, cmd.Stderr = f, f
		err := cmd.Run()
		f.Close()
		status := "SUCCESS"
		if err != nil {
			status = "FAILED"
		}
		return mcp.NewToolResultText(fmt.Sprintf("Preset %s finished: %s.", name, status)), nil
	})
}
