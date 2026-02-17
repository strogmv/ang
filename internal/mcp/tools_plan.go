package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/strogmv/ang/compiler"
)

func registerPlanTools(addTool toolAdder) {
	addTool("ang_plan", mcp.NewTool("ang_plan",
		mcp.WithDescription("Create a structured architecture plan from a natural-language goal and current CUE intent."),
		mcp.WithString("goal", mcp.Required()),
		mcp.WithBoolean("verify_determinism", mcp.Description("Forecast expected artifact hashes and determinism risks before apply.")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		goal := strings.TrimSpace(mcp.ParseString(request, "goal", ""))
		if goal == "" {
			return mcp.NewToolResultText(`{"status":"invalid","message":"goal is required"}`), nil
		}
		verifyDeterminism := mcp.ParseBoolean(request, "verify_determinism", false)
		plan, err := buildGoalPlan(goal)
		if err != nil {
			return mcp.NewToolResultText((&ANGReport{
				Status:      "Failed",
				Summary:     []string{"Unable to build plan from current intent."},
				NextActions: []string{"Fix CUE validation errors and retry ang_plan"},
				Artifacts:   map[string]string{"error": err.Error()},
			}).ToJSON()), nil
		}
		if verifyDeterminism {
			forecast, ferr := verifyPlanDeterminism(plan)
			if ferr != nil {
				plan["determinism"] = map[string]any{
					"verified":      false,
					"risk_flags":    []string{"determinism_forecast_failed"},
					"drift_reasons": []string{ferr.Error()},
				}
			} else {
				plan["determinism"] = forecast
			}
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

type artifactHashRecord struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

type artifactHashManifest struct {
	Artifacts []artifactHashRecord `json:"artifacts"`
}

func verifyPlanDeterminism(plan map[string]any) (map[string]any, error) {
	patches := extractPlanPatches(plan)
	risks := assessDeterminismRiskFlags(patches)
	baseline, baselineErr := readArtifactHashes(".")

	resp := map[string]any{
		"verified":                false,
		"patches":                 len(patches),
		"risk_flags":              risks,
		"drift_reasons":           []string{},
		"expected_hashes":         []artifactHashRecord{},
		"baseline_manifest_found": baselineErr == nil,
	}
	if baselineErr != nil {
		resp["drift_reasons"] = []string{"baseline manifest missing: run ang build once before determinism verification"}
	}

	if len(patches) == 0 {
		if baselineErr == nil {
			resp["expected_hashes"] = baseline
			resp["verified"] = true
		}
		return resp, nil
	}

	tmpDir, err := os.MkdirTemp("", "ang-mcp-determinism-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := copyProjectTree(".", tmpDir); err != nil {
		return nil, fmt.Errorf("copy project: %w", err)
	}

	patchFailures := []string{}
	for _, p := range patches {
		path := strings.TrimSpace(anyToString(p["path"]))
		if path == "" {
			patchFailures = append(patchFailures, "patch has empty path")
			continue
		}
		selector := strings.TrimSpace(anyToString(p["selector"]))
		content := anyToString(p["content"])
		force := anyToBool(p["forced_merge"])

		targetPath := filepath.Join(tmpDir, filepath.FromSlash(path))
		merged, err := GetMergedContent(targetPath, selector, content, force)
		if err != nil {
			patchFailures = append(patchFailures, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			patchFailures = append(patchFailures, fmt.Sprintf("%s: mkdir failed: %v", path, err))
			continue
		}
		if err := os.WriteFile(targetPath, merged, 0o644); err != nil {
			patchFailures = append(patchFailures, fmt.Sprintf("%s: write failed: %v", path, err))
			continue
		}
	}
	if len(patchFailures) > 0 {
		return map[string]any{
			"verified":      false,
			"patches":       len(patches),
			"risk_flags":    appendUnique(risks, "patch_apply_failed"),
			"drift_reasons": patchFailures,
		}, nil
	}

	manifest1, out1, err := runBuildAndReadManifest(tmpDir)
	if err != nil {
		return map[string]any{
			"verified":      false,
			"patches":       len(patches),
			"risk_flags":    appendUnique(risks, "forecast_build_failed"),
			"drift_reasons": []string{fmt.Sprintf("forecast build failed: %v", err), truncate(out1, 600)},
		}, nil
	}
	manifest2, out2, err := runBuildAndReadManifest(tmpDir)
	if err != nil {
		return map[string]any{
			"verified":      false,
			"patches":       len(patches),
			"risk_flags":    appendUnique(risks, "forecast_build_failed"),
			"drift_reasons": []string{fmt.Sprintf("second forecast build failed: %v", err), truncate(out2, 600)},
		}, nil
	}

	driftReasons := []string{}
	if !sameHashRecords(manifest1, manifest2) {
		risks = appendUnique(risks, "nondeterministic_forecast_output")
		driftReasons = append(driftReasons, "same planned patch set produced different artifact hashes across two forecast builds")
	}
	if baselineErr == nil {
		driftReasons = append(driftReasons, diffHashReasons(baseline, manifest1)...)
	}

	return map[string]any{
		"verified":                len(driftReasons) == 0 || (len(driftReasons) > 0 && !containsString(driftReasons, "different artifact hashes across two forecast builds")),
		"patches":                 len(patches),
		"baseline_manifest_found": baselineErr == nil,
		"expected_hashes":         manifest1,
		"risk_flags":              risks,
		"drift_reasons":           driftReasons,
	}, nil
}

func runBuildAndReadManifest(projectDir string) ([]artifactHashRecord, string, error) {
	cmd := exec.Command(resolveANGExecutable(), "build", "--mode=release", "--phase=all")
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, string(out), err
	}
	hashes, readErr := readArtifactHashes(projectDir)
	return hashes, string(out), readErr
}

func readArtifactHashes(projectRoot string) ([]artifactHashRecord, error) {
	path := filepath.Join(projectRoot, ".ang", "cache", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m artifactHashManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	sort.Slice(m.Artifacts, func(i, j int) bool { return m.Artifacts[i].Path < m.Artifacts[j].Path })
	return m.Artifacts, nil
}

func assessDeterminismRiskFlags(patches []map[string]any) []string {
	risks := []string{}
	if len(patches) > 10 {
		risks = append(risks, "large_patch_batch")
	}
	for _, p := range patches {
		path := strings.ToLower(strings.TrimSpace(anyToString(p["path"])))
		selector := strings.TrimSpace(anyToString(p["selector"]))
		content := strings.ToLower(anyToString(p["content"]))
		if selector == "" {
			risks = append(risks, "root_level_merge")
		}
		if strings.Contains(path, "cue/project") || strings.Contains(path, "cue/targets") {
			risks = append(risks, "build_target_change")
		}
		if strings.Contains(content, "map[") || strings.Contains(content, "{") && strings.Contains(content, ":") {
			risks = append(risks, "unordered_map_semantics")
		}
	}
	if len(risks) == 0 {
		risks = append(risks, "low_risk")
	}
	return uniqueSortedStrings(risks)
}

func diffHashReasons(base, next []artifactHashRecord) []string {
	baseMap := make(map[string]string, len(base))
	nextMap := make(map[string]string, len(next))
	for _, h := range base {
		baseMap[h.Path] = h.Hash
	}
	for _, h := range next {
		nextMap[h.Path] = h.Hash
	}
	added := []string{}
	removed := []string{}
	changed := []string{}
	for path, hash := range nextMap {
		if old, ok := baseMap[path]; !ok {
			added = append(added, path)
		} else if old != hash {
			changed = append(changed, path)
		}
	}
	for path := range baseMap {
		if _, ok := nextMap[path]; !ok {
			removed = append(removed, path)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	reasons := []string{}
	if len(added) > 0 {
		reasons = append(reasons, fmt.Sprintf("artifacts added: %d (e.g. %s)", len(added), strings.Join(added[:min(3, len(added))], ", ")))
	}
	if len(removed) > 0 {
		reasons = append(reasons, fmt.Sprintf("artifacts removed: %d (e.g. %s)", len(removed), strings.Join(removed[:min(3, len(removed))], ", ")))
	}
	if len(changed) > 0 {
		reasons = append(reasons, fmt.Sprintf("artifacts hash-changed: %d (e.g. %s)", len(changed), strings.Join(changed[:min(3, len(changed))], ", ")))
	}
	return reasons
}

func copyProjectTree(srcRoot, dstRoot string) error {
	return filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(rel, ".git") || strings.HasPrefix(rel, ".ang") || strings.HasPrefix(rel, "dist") || strings.HasPrefix(rel, "node_modules") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dst := filepath.Join(dstRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, info.Mode())
	})
}

func sameHashRecords(a, b []artifactHashRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func appendUnique(items []string, v string) []string {
	for _, item := range items {
		if item == v {
			return items
		}
	}
	return append(items, v)
}

func uniqueSortedStrings(in []string) []string {
	set := make(map[string]struct{}, len(in))
	for _, v := range in {
		if strings.TrimSpace(v) == "" {
			continue
		}
		set[v] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func containsString(arr []string, needle string) bool {
	for _, v := range arr {
		if strings.Contains(v, needle) {
			return true
		}
	}
	return false
}

func anyToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprint(v)
	}
}

func anyToBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(strings.TrimSpace(x), "true")
	default:
		return false
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
