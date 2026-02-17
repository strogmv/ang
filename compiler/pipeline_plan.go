package compiler

import (
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/strogmv/ang/compiler/normalizer"
	planpkg "github.com/strogmv/ang/compiler/plan"
)

func BuildPlan(basePath string, opts RunOptions) (*planpkg.BuildPlan, error) {
	start := time.Now()
	basePath = filepath.Clean(basePath)

	plan := &planpkg.BuildPlan{
		SchemaVersion:  SchemaVersion,
		PlanVersion:    Version,
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		WorkspaceRoot:  basePath,
		BuildArgs: planpkg.BuildArgs{
			AutoApply: opts.Phase != PhasePlan,
		},
		Status: planpkg.StatusOK,
		Steps: []planpkg.PlanStep{
			{Name: "parse+normalize+ir", Status: "ok"},
			{Name: "diff", Status: "ok", Message: "PR1 placeholder: diff not yet computed"},
		},
	}

	projectHash, _ := ComputeProjectHash(basePath)
	plan.InputHash = projectHash
	workspaceHash, _ := planpkg.ComputeWorkspaceHash(basePath)
	plan.Preconditions.WorkspaceHash = workspaceHash
	plan.Preconditions.GoVersion = runtime.Version()

	_, _, _, _, _, _, _, _, err := RunPipelineWithOptions(basePath, PipelineOptions{
		WarningSink: opts.WarningSink,
	})
	for _, w := range LatestDiagnostics {
		plan.Diagnostics = append(plan.Diagnostics, warningToPlanDiagnostic(w))
		if isErrorWarning(w) && plan.Status != planpkg.StatusFail {
			plan.Status = planpkg.StatusFail
		} else if plan.Status == planpkg.StatusOK {
			plan.Status = planpkg.StatusWarn
		}
	}
	if err != nil {
		plan.Diagnostics = append(plan.Diagnostics, planpkg.Diagnostic{
			Level:   "error",
			Code:    "PIPELINE_ERROR",
			Message: err.Error(),
		})
		plan.Status = planpkg.StatusFail
	}

	plan.Steps[0].DurationMs = time.Since(start).Milliseconds()
	plan.Summary = summarizeChanges(plan.Changes)
	return plan, nil
}

func warningToPlanDiagnostic(w normalizer.Warning) planpkg.Diagnostic {
	level := strings.ToLower(strings.TrimSpace(w.Severity))
	if level == "" {
		level = "warn"
	}
	return planpkg.Diagnostic{
		Level:   level,
		Code:    w.Code,
		Message: w.Message,
		File:    w.File,
		Line:    w.Line,
	}
}

func isErrorWarning(w normalizer.Warning) bool {
	return strings.EqualFold(strings.TrimSpace(w.Severity), "error")
}

func summarizeChanges(changes []planpkg.FileChange) planpkg.PlanSummary {
	var s planpkg.PlanSummary
	for _, c := range changes {
		switch strings.ToLower(c.Op) {
		case "add":
			s.Add++
		case "update":
			s.Update++
		case "delete":
			s.Delete++
		}
	}
	return s
}
