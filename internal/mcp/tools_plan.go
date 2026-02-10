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
	"github.com/strogmv/ang/compiler"
)

func registerPlanTools(addTool toolAdder) {
	addTool("ang_plan", mcp.NewTool("ang_plan",
		mcp.WithDescription("Create a structured architecture plan from a natural-language goal and current CUE intent."),
		mcp.WithString("goal", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		goal := strings.TrimSpace(mcp.ParseString(request, "goal", ""))
		if goal == "" {
			return mcp.NewToolResultText(`{"status":"invalid","message":"goal is required"}`), nil
		}
		plan, err := buildGoalPlan(goal)
		if err != nil {
			return mcp.NewToolResultText((&ANGReport{
				Status:      "Failed",
				Summary:     []string{"Unable to build plan from current intent."},
				NextActions: []string{"Fix CUE validation errors and retry ang_plan"},
				Artifacts:   map[string]string{"error": err.Error()},
			}).ToJSON()), nil
		}
		b, _ := json.MarshalIndent(plan, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_doctor", mcp.NewTool("ang_doctor",
		mcp.WithDescription("Analyze build logs and suggest fixes."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logData, _ := os.ReadFile("ang-build.log")
		log := string(logData)
		resp := buildDoctorResponse(log)
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_do", mcp.NewTool("ang_do",
		mcp.WithDescription("High-level intent tool: plan, apply CUE patches, build, and return doctor report."),
		mcp.WithString("intent", mcp.Required()),
		mcp.WithBoolean("auto_apply", mcp.Description("Apply generated CUE patches automatically (default true).")),
		mcp.WithBoolean("run_build", mcp.Description("Run build after patch apply (default true).")),
		mcp.WithBoolean("auto_undo_on_fail", mcp.Description("Rollback applied patches when apply/build fails (default true).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		intent := strings.TrimSpace(mcp.ParseString(request, "intent", ""))
		if intent == "" {
			return mcp.NewToolResultText(`{"status":"invalid","message":"intent is required"}`), nil
		}
		autoApply := mcp.ParseBoolean(request, "auto_apply", true)
		runBuild := mcp.ParseBoolean(request, "run_build", true)
		autoUndoOnFail := mcp.ParseBoolean(request, "auto_undo_on_fail", true)

		plan, err := buildGoalPlan(intent)
		if err != nil {
			return mcp.NewToolResultText((&ANGReport{
				Status:      "Failed",
				Summary:     []string{"Unable to build plan from current intent."},
				NextActions: []string{"Fix CUE validation errors and retry ang_do"},
				Artifacts:   map[string]string{"error": err.Error()},
			}).ToJSON()), nil
		}

		patches := extractPlanPatches(plan)
		resp := map[string]any{
			"status":            "planned",
			"intent":            intent,
			"auto_apply":        autoApply,
			"run_build":         runBuild,
			"auto_undo_on_fail": autoUndoOnFail,
			"patches_available": len(patches),
			"plan":              plan,
		}

		if !autoApply || len(patches) == 0 {
			b, _ := json.MarshalIndent(resp, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		}

		applied := make([]map[string]any, 0, len(patches))
		failed := make([]map[string]any, 0)
		backups := map[string][]byte{}
		for _, p := range patches {
			path, _ := p["path"].(string)
			selector, _ := p["selector"].(string)
			content, _ := p["content"].(string)
			force, _ := p["forced_merge"].(bool)

			if err := validateCuePath(path); err != nil {
				failed = append(failed, map[string]any{"path": path, "error": err.Error()})
				continue
			}

			merged, err := GetMergedContent(path, selector, content, force)
			if err != nil {
				failed = append(failed, map[string]any{"path": path, "error": err.Error()})
				continue
			}

			orig, _ := os.ReadFile(path)
			if _, exists := backups[path]; !exists {
				backups[path] = append([]byte(nil), orig...)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				failed = append(failed, map[string]any{"path": path, "error": err.Error()})
				continue
			}
			if err := os.WriteFile(path, merged, 0o644); err != nil {
				failed = append(failed, map[string]any{"path": path, "error": err.Error()})
				continue
			}

			vet := exec.Command("cue", "vet", "./"+filepath.Dir(path))
			if out, err := vet.CombinedOutput(); err != nil {
				_ = os.WriteFile(path, orig, 0o644)
				failed = append(failed, map[string]any{"path": path, "error": fmt.Sprintf("cue vet failed: %s", string(out))})
				continue
			}
			if _, _, _, _, _, _, _, _, err := compiler.RunPipeline("."); err != nil {
				_ = os.WriteFile(path, orig, 0o644)
				failed = append(failed, map[string]any{"path": path, "error": fmt.Sprintf("pipeline failed: %v", err)})
				continue
			}

			applied = append(applied, map[string]any{
				"path":     path,
				"selector": selector,
			})
		}

		resp["status"] = "applied"
		resp["patches_applied"] = len(applied)
		resp["patches_failed"] = failed
		resp["applied"] = applied
		applyFailed := len(failed) > 0

		buildFailed := false
		if runBuild {
			cmd := exec.Command(resolveANGExecutable(), "build")
			out, err := cmd.CombinedOutput()
			buildLog := string(out)
			buildStatus := "success"
			if err != nil || strings.Contains(buildLog, "Build FAILED") {
				buildStatus = "failed"
				buildFailed = true
			}
			_ = os.WriteFile("ang-build.log", []byte(buildLog), 0o644)
			resp["build_status"] = buildStatus
			resp["build_log_excerpt"] = truncate(buildLog, 4000)
			resp["doctor"] = buildDoctorResponse(buildLog)
			if diffOut, err := exec.Command("git", "diff", "--shortstat").CombinedOutput(); err == nil {
				resp["diff_summary"] = strings.TrimSpace(string(diffOut))
			}
		}

		if autoUndoOnFail && (applyFailed || buildFailed) && len(backups) > 0 {
			rollbackErrs := rollbackCueBackups(backups)
			resp["rolled_back"] = len(rollbackErrs) == 0
			resp["rollback_files"] = len(backups)
			if len(rollbackErrs) > 0 {
				resp["rollback_errors"] = rollbackErrs
			}
		}

		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func rollbackCueBackups(backups map[string][]byte) []string {
	errs := []string{}
	paths := make([]string, 0, len(backups))
	for p := range backups {
		paths = append(paths, p)
	}
	for _, p := range paths {
		if err := os.WriteFile(p, backups[p], 0o644); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p, err))
		}
	}
	return errs
}

func extractPlanPatches(plan map[string]any) []map[string]any {
	root, ok := plan["plan"].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := root["cue_apply_patch"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]map[string]any)
	if ok {
		return arr
	}
	anyArr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(anyArr))
	for _, item := range anyArr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}
