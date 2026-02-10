package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		changed := !bytes.Equal(orig, newContent)
		diffText := unifiedDiff(orig, newContent)
		resp := map[string]any{
			"status":         "ok",
			"path":           path,
			"selector":       selector,
			"forced_merge":   force,
			"changed":        changed,
			"bytes_before":   len(orig),
			"bytes_after":    len(newContent),
			"before_preview": previewText(string(orig), 1200),
			"after_preview":  previewText(string(newContent), 1200),
			"diff_unified":   previewText(diffText, 6000),
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
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
		logData, _ := os.ReadFile(logFile)
		logText := string(logData)
		resp := map[string]any{
			"status": status,
			"preset": name,
		}
		if status == "FAILED" {
			resp["log_tail"] = tailLines(logText, 30)
			resp["doctor"] = buildDoctorResponse(logText)
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func previewText(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n...<truncated>"
}

func unifiedDiff(before, after []byte) string {
	tmpDir, err := os.MkdirTemp("", "ang-cue-diff-*")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(tmpDir)

	beforePath := filepath.Join(tmpDir, "before.cue")
	afterPath := filepath.Join(tmpDir, "after.cue")
	_ = os.WriteFile(beforePath, before, 0o644)
	_ = os.WriteFile(afterPath, after, 0o644)

	cmd := exec.Command("git", "--no-pager", "diff", "--no-index", "--", beforePath, afterPath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	// git diff exits with code 1 when differences exist; this is expected.
	if len(out) > 0 {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func tailLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if len(lines) <= n {
		return strings.TrimRight(strings.Join(lines, "\n"), "\n")
	}
	start := len(lines) - n
	return strings.TrimRight(strings.Join(lines[start:], "\n"), "\n")
}
