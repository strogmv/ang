package compiler

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	planpkg "github.com/strogmv/ang/compiler/plan"
)

const (
	Version       = "0.1.126"
	SchemaVersion = "1"
)

func ComputeProjectHash(path string) (string, error) {
	h := sha256.New()
	err := filepath.Walk(filepath.Join(path, "cue"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".cue") {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(h, f); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

type PipelineOptions struct {
	WarningSink       func(normalizer.Warning)
	ArchitectureMode  string
	AllowCrossService map[string]map[string]struct{}
}

type RunPhase string

const (
	PhaseAll   RunPhase = "all"
	PhasePlan  RunPhase = "plan"
	PhaseApply RunPhase = "apply"
)

type RunOptions struct {
	Phase       RunPhase
	PlanFile    string
	JSON        bool
	OutPlan     string
	WarningSink func(normalizer.Warning)
}

var LatestDiagnostics []normalizer.Warning

func RunWithOptions(basePath string, opts RunOptions) (*planpkg.BuildPlan, error) {
	phase := opts.Phase
	if phase == "" {
		phase = PhaseAll
	}

	var currentPlan *planpkg.BuildPlan
	var err error
	switch phase {
	case PhasePlan:
		currentPlan, err = BuildPlan(basePath, opts)
		if err != nil {
			return nil, err
		}
		if opts.OutPlan != "" {
			if err := planpkg.WritePlan(opts.OutPlan, currentPlan); err != nil {
				return nil, err
			}
		}
		return currentPlan, nil
	case PhaseApply:
		if opts.PlanFile == "" {
			return nil, fmt.Errorf("--plan-file is required for apply phase")
		}
		currentPlan, err = planpkg.ReadPlan(opts.PlanFile)
		if err != nil {
			return nil, err
		}
		if err := ApplyPlan(basePath, currentPlan); err != nil {
			return nil, err
		}
		return currentPlan, nil
	case PhaseAll:
		currentPlan, err = BuildPlan(basePath, opts)
		if err != nil {
			return nil, err
		}
		if opts.OutPlan != "" {
			if err := planpkg.WritePlan(opts.OutPlan, currentPlan); err != nil {
				return nil, err
			}
		}
		if err := ApplyPlan(basePath, currentPlan); err != nil {
			return nil, err
		}
		return currentPlan, nil
	default:
		return nil, fmt.Errorf("unknown run phase: %s", phase)
	}
}

func RunPipeline(basePath string) ([]normalizer.Entity, []normalizer.Service, []normalizer.Endpoint, []normalizer.Repository, []normalizer.EventDef, []normalizer.ErrorDef, []normalizer.ScheduleDef, []normalizer.ScenarioDef, []normalizer.ScopeDef, error) {
	return RunPipelineWithOptions(basePath, PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			LatestDiagnostics = append(LatestDiagnostics, w)
		},
	})
}

func RunPipelineWithOptions(basePath string, opts PipelineOptions) ([]normalizer.Entity, []normalizer.Service, []normalizer.Endpoint, []normalizer.Repository, []normalizer.EventDef, []normalizer.ErrorDef, []normalizer.ScheduleDef, []normalizer.ScenarioDef, []normalizer.ScopeDef, error) {
	normalized, err := RunSemanticPhasesWithOptions(basePath, opts)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}

	return normalized.Entities, normalized.Services, normalized.Endpoints, normalized.Repos, normalized.Events, normalized.Errors, normalized.Schedules, normalized.Scenarios, normalized.Scopes, nil
}

func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}
	var out []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, '_')
		}
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		out = append(out, r)
	}
	return string(out)
}

func parseSourcePos(source string) (file string, line int) {
	file = source
	if source == "" {
		return "", 0
	}
	idx := strings.LastIndex(source, ":")
	if idx <= 0 || idx == len(source)-1 {
		return source, 0
	}
	n, err := strconv.Atoi(source[idx+1:])
	if err != nil {
		return source, 0
	}
	return source[:idx], n
}
