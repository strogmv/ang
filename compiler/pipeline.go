package compiler

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
	planpkg "github.com/strogmv/ang/compiler/plan"
	"github.com/strogmv/ang/compiler/transformers"
)

const (
	Version       = "0.1.116"
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

// emitEventUsageDiagnostics surfaces dead/unused events as warnings (non-fatal).
// - dead event: defined but never published or subscribed
// - orphan publish: published but nobody subscribes
// - missing publisher: subscribed but no publisher exists
func emitEventUsageDiagnostics(services []normalizer.Service, events []normalizer.EventDef, schedules []normalizer.ScheduleDef, broadcastOnly map[string]struct{}, planned map[string]struct{}, opts PipelineOptions) {
	defined := make(map[string]struct{})
	for _, e := range events {
		defined[e.Name] = struct{}{}
	}

	published := make(map[string]struct{})
	subscribed := make(map[string]struct{})

	for _, s := range services {
		for _, evt := range s.Publishes {
			published[evt] = struct{}{}
		}
		for evt := range s.Subscribes {
			subscribed[evt] = struct{}{}
		}
		for _, m := range s.Methods {
			for _, evt := range m.Publishes {
				published[evt] = struct{}{}
			}
		}
	}
	for _, sch := range schedules {
		if sch.Publish != "" {
			published[sch.Publish] = struct{}{}
		}
	}

	for name := range defined {
		if _, okPub := published[name]; okPub {
			continue
		}
		if _, okSub := subscribed[name]; okSub {
			continue
		}
		if _, ok := planned[name]; ok {
			continue
		}
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "dead-event",
			Code:     "DEAD_EVENT",
			Severity: "warn",
			Message:  fmt.Sprintf("Event %s is defined but never published or subscribed", name),
		}, opts)
	}

	for name := range published {
		if _, ok := subscribed[name]; ok {
			continue
		}
		if _, ok := broadcastOnly[name]; ok {
			continue
		}
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "orphan-publish",
			Code:     "ORPHAN_PUBLISH",
			Severity: "warn",
			Message:  fmt.Sprintf("Event %s is published but has no subscribers", name),
		}, opts)
	}

	for name := range subscribed {
		if _, ok := published[name]; ok {
			continue
		}
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "missing-publisher",
			Code:     "MISSING_PUBLISH",
			Severity: "warn",
			Message:  fmt.Sprintf("Event %s is subscribed but never published", name),
		}, opts)
	}
}

// loadEventAnnotations reads cue/events/annotations.cue (optional) and returns
// two sets: broadcastOnly and planned.
func loadEventAnnotations(basePath string) (map[string]struct{}, map[string]struct{}) {
	broadcastOnly := make(map[string]struct{})
	planned := make(map[string]struct{})

	path := filepath.Join(basePath, "cue", "events_meta", "annotations.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		return broadcastOnly, planned
	}

	// very small ad-hoc parser for the two maps; avoid pulling CUE parser here
	var current string
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "//") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "BroadcastOnly:") {
			current = "broadcast"
			continue
		}
		if strings.HasPrefix(line, "Planned:") {
			current = "planned"
			continue
		}
		if strings.HasSuffix(line, "{") || strings.HasSuffix(line, "}") {
			continue
		}
		name := strings.TrimSpace(line)
		name = strings.TrimSuffix(name, "true")
		name = strings.TrimSuffix(name, ":")
		name = strings.TrimSpace(strings.TrimSuffix(name, ":"))
		name = strings.TrimSpace(strings.TrimSuffix(name, ":"))
		name = strings.TrimSpace(strings.TrimSuffix(name, ":"))
		name = strings.TrimSuffix(name, "=")
		name = strings.TrimSpace(strings.TrimSuffix(name, ""))
		if name == "" {
			continue
		}
		if current == "broadcast" {
			broadcastOnly[name] = struct{}{}
		} else if current == "planned" {
			planned[name] = struct{}{}
		}
	}
	return broadcastOnly, planned
}

func emitSelectProjectionDiagnostics(
	entity string,
	finders []normalizer.RepositoryFinder,
	entityFieldMap map[string]map[string]string,
	opts PipelineOptions,
) {
	fields, ok := entityFieldMap[entity]
	if !ok || len(fields) == 0 {
		return
	}
	total := len(fields)

	for _, f := range finders {
		if len(f.Select) == 0 {
			continue
		}
		if !finderReturnsEntity(f, entity) {
			continue
		}

		selected := make(map[string]struct{}, len(f.Select))
		for _, col := range f.Select {
			key := canonicalFieldToken(col)
			if key != "" {
				selected[key] = struct{}{}
			}
		}

		var missing []string
		for fieldName := range fields {
			if _, ok := selected[canonicalFieldToken(fieldName)]; !ok {
				missing = append(missing, fieldName)
			}
		}
		if len(missing) == 0 {
			continue
		}

		file, line := parseSourcePos(f.Source)
		msg := fmt.Sprintf(
			"Finder '%s.%s' returns domain.%s but select has %d/%d fields; partial entity select is forbidden",
			entity, f.Name, entity, total-len(missing), total,
		)
		hint := "Use full select for entity return, or set return_type to a projection DTO/custom type"
		diag := normalizer.Warning{
			Kind:     "architecture",
			Code:     "ENTITY_PARTIAL_SELECT_ERROR",
			Severity: "error",
			Message:  msg,
			File:     file,
			Line:     line,
			Hint:     hint,
		}
		recordPipelineDiagnostic(diag, opts)
	}
}

func synthesizeImplicitProjections(
	entity string,
	finders []normalizer.RepositoryFinder,
	entityByName map[string]normalizer.Entity,
	projNameByKey map[string]string,
) ([]normalizer.RepositoryFinder, []normalizer.Entity) {
	src, ok := entityByName[entity]
	if !ok {
		return finders, nil
	}

	lookup := make(map[string]normalizer.Field)
	for _, f := range src.Fields {
		if f.SkipDomain {
			continue
		}
		lookup[canonicalFieldToken(f.Name)] = f
	}

	var projections []normalizer.Entity
	for i := range finders {
		f := &finders[i]
		if len(f.Select) == 0 || !finderReturnsEntity(*f, entity) {
			continue
		}
		if strings.TrimSpace(f.ReturnType) != "" {
			continue // Explicit return_type must remain explicit; validator will enforce compatibility.
		}

		fields, orderedCols, key, ok := projectionFieldsForSelect(entity, f.Select, src)
		if !ok {
			continue
		}
		if len(fields) == len(lookup) {
			continue // Full select is allowed for domain entity return.
		}

		projName, ok := projNameByKey[key]
		if !ok {
			projName = projectionName(entity, orderedCols)
			projNameByKey[key] = projName
			projections = append(projections, normalizer.Entity{
				Name:   projName,
				Owner:  src.Owner,
				Fields: fields,
				Metadata: map[string]any{
					"projection": true,
					"source":     entity,
				},
				Source: f.Source,
			})
		}

		f.Select = append([]string(nil), orderedCols...)
		if finderReturnsMany(*f, entity) {
			f.ReturnType = "[]domain." + projName
		} else {
			f.ReturnType = "*domain." + projName
		}
	}
	return finders, projections
}

func projectionFieldsForSelect(
	entity string,
	selectCols []string,
	src normalizer.Entity,
) ([]normalizer.Field, []string, string, bool) {
	keys := make([]string, 0, len(selectCols))
	seen := make(map[string]struct{}, len(selectCols))
	for _, col := range selectCols {
		k := canonicalFieldToken(col)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil, nil, "", false
	}
	keyTokens := append([]string(nil), keys...)
	sort.Strings(keyTokens)

	fields := make([]normalizer.Field, 0, len(keys))
	orderedCols := make([]string, 0, len(keys))
	for _, f := range src.Fields {
		if f.SkipDomain {
			continue
		}
		k := canonicalFieldToken(f.Name)
		if _, ok := seen[k]; !ok {
			continue
		}
		delete(seen, k)
		fields = append(fields, f)
		orderedCols = append(orderedCols, f.Name)
	}
	if len(seen) > 0 {
		return nil, nil, "", false
	}
	return fields, orderedCols, entity + "|" + strings.Join(keyTokens, ","), true
}

func projectionName(entity string, sortedCols []string) string {
	parts := make([]string, 0, len(sortedCols))
	for _, c := range sortedCols {
		s := strings.TrimSpace(c)
		if s == "" {
			continue
		}
		parts = append(parts, strings.ToLower(ToSnakeCase(s)))
	}
	if len(parts) == 0 {
		return entity + "_Proj"
	}
	return entity + "_" + strings.Join(parts, "_") + "_Proj"
}

func finderReturnsEntity(f normalizer.RepositoryFinder, entity string) bool {
	if strings.EqualFold(f.Returns, "count") || strings.EqualFold(f.Action, "delete") {
		return false
	}
	if f.ReturnType == "" {
		return true
	}

	rt := strings.TrimSpace(strings.ToLower(f.ReturnType))
	rt = strings.TrimPrefix(rt, "[]")
	rt = strings.TrimPrefix(rt, "*")
	rt = strings.TrimPrefix(rt, "domain.")
	return rt == strings.ToLower(entity)
}

func finderReturnsMany(f normalizer.RepositoryFinder, entity string) bool {
	if strings.EqualFold(f.Returns, "many") || strings.EqualFold(f.Returns, "[]"+entity) {
		return true
	}
	rt := strings.TrimSpace(strings.ToLower(f.ReturnType))
	return strings.HasPrefix(rt, "[]")
}

func canonicalFieldToken(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return ""
	}
	s = strings.Trim(s, "`\"")
	s = strings.SplitN(s, " ", 2)[0]
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		s = s[idx+1:]
	}
	s = strings.ToLower(s)
	return strings.ReplaceAll(s, "_", "")
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

func emitFSMIntegrityDiagnostics(entities []normalizer.Entity, opts PipelineOptions) {
	seen := map[string]struct{}{}
	for _, e := range entities {
		if e.FSM == nil {
			continue
		}

		stateSet := make(map[string]struct{}, len(e.FSM.States))
		for _, s := range e.FSM.States {
			key := strings.TrimSpace(s)
			if key == "" {
				continue
			}
			stateSet[key] = struct{}{}
		}
		if len(stateSet) == 0 {
			continue
		}

		for from, toStates := range e.FSM.Transitions {
			fromState := strings.TrimSpace(from)
			if fromState != "" {
				if _, ok := stateSet[fromState]; !ok {
					file, line := parseSourcePos(e.Source)
					toPreview := ""
					if len(toStates) > 0 {
						toPreview = strings.TrimSpace(toStates[0])
					}
					diag := normalizer.Warning{
						Kind:     "architecture",
						Code:     "E_FSM_UNDEFINED_STATE",
						Severity: "error",
						Message: fmt.Sprintf(
							"Entity '%s' FSM transition '%s→%s' references undefined state '%s'",
							e.Name, fromState, toPreview, fromState,
						),
						File: file,
						Line: line,
						Hint: fmt.Sprintf("Add '%s' to fsm.states or update transition source.", fromState),
					}
					key := diag.Code + "|" + diag.Message + "|" + diag.File + "|" + strconv.Itoa(diag.Line)
					if _, ok := seen[key]; !ok {
						seen[key] = struct{}{}
						recordPipelineDiagnostic(diag, opts)
					}
				}
			}
			for _, to := range toStates {
				state := strings.TrimSpace(to)
				if state == "" {
					continue
				}
				if _, ok := stateSet[state]; ok {
					continue
				}
				file, line := parseSourcePos(e.Source)
				diag := normalizer.Warning{
					Kind:     "architecture",
					Code:     "E_FSM_UNDEFINED_STATE",
					Severity: "error",
					Message: fmt.Sprintf(
						"Entity '%s' FSM transition '%s→%s' references undefined state '%s'",
						e.Name, from, state, state,
					),
					File: file,
					Line: line,
					Hint: fmt.Sprintf("Add '%s' to fsm.states or update transition target.", state),
				}
				key := diag.Code + "|" + diag.Message + "|" + diag.File + "|" + strconv.Itoa(diag.Line)
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					recordPipelineDiagnostic(diag, opts)
				}
			}
		}
	}
}

func recordPipelineDiagnostic(diag normalizer.Warning, opts PipelineOptions) {
	if opts.WarningSink != nil {
		opts.WarningSink(diag)
		return
	}
	LatestDiagnostics = append(LatestDiagnostics, diag)
}

func emitFileSizeDiagnostics(path string, opts PipelineOptions) {
	const lineLimit = 300
	matches, _ := filepath.Glob(filepath.Join(path, "*.cue"))
	for _, filePath := range matches {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		lineCount := strings.Count(string(content), "\n") + 1
		if lineCount > lineLimit {
			diag := normalizer.Warning{
				Kind:     "file-size",
				Code:     "LARGE_CUE_FILE",
				Severity: "warn",
				Message:  fmt.Sprintf("CUE file has %d lines (recommended limit: %d)", lineCount, lineLimit),
				File:     filePath,
				Line:     1,
				Hint:     "Split into multiple files in the same directory (CUE merges files with same package automatically)",
			}
			recordPipelineDiagnostic(diag, opts)
		}
	}
}

func LoadOptionalDomain(p *parser.Parser, path string) (cue.Value, bool, error) {
	matches, _ := filepath.Glob(filepath.Join(path, "*.cue"))
	if len(matches) == 0 {
		return cue.Value{}, false, nil
	}
	val, err := p.LoadDomain(path)
	if err != nil {
		return cue.Value{}, false, err
	}
	return val, true, nil
}

func ConvertAndTransform(
	entities []normalizer.Entity, services []normalizer.Service, events []normalizer.EventDef,
	errors []normalizer.ErrorDef, endpoints []normalizer.Endpoint, scopes []normalizer.ScopeDef, repos []normalizer.Repository,
	config normalizer.ConfigDef, auth *normalizer.AuthDef, rbac *normalizer.RBACDef,
	schedules []normalizer.ScheduleDef, views []normalizer.ViewDef, project normalizer.ProjectDef,
) (*ir.Schema, error) {
	schema := ir.ConvertFromNormalizer(entities, services, events, errors, endpoints, scopes, repos, config, auth, rbac, schedules, views, project)
	if err := ir.MigrateToCurrent(schema); err != nil {
		return nil, WrapContractError(StageIR, ErrCodeIRVersionMigration, "migrate ir schema", err)
	}

	registry := transformers.DefaultRegistry()
	if err := registry.Apply(schema); err != nil {
		return nil, WrapContractError(StageTransformers, ErrCodeTransformerApply, "apply transformers", err)
	}

	hooks := transformers.DefaultHookRegistry()
	if err := hooks.Process(schema); err != nil {
		return nil, WrapContractError(StageTransformers, ErrCodeHookProcess, "process hooks", err)
	}
	if err := ir.ValidateABIV2(schema); err != nil {
		return nil, WrapContractError(StageIR, ErrCodeIRVersionMigration, "validate ir abi v2", err)
	}

	return schema, nil
}

func validateFlowIntegrity(services []normalizer.Service) error {
	for _, svc := range services {
		for _, m := range svc.Methods {
			if len(m.Flow) == 0 {
				continue
			}
			declared := make(map[string]string)
			used := make(map[string]bool)
			for _, s := range m.Flow {
				for _, arg := range []string{"input", "value", "condition", "payload", "actor", "company"} {
					if val, ok := s.Args[arg].(string); ok {
						for name := range declared {
							if strings.Contains(val, name) {
								used[name] = true
							}
						}
					}
				}
				if out, ok := s.Args["output"].(string); ok && out != "" && out != "resp" {
					declared[out] = fmt.Sprintf("%s:%d", s.File, s.Line)
				}
			}
			for name, loc := range declared {
				if !used[name] {
					return fmt.Errorf("Logic Error: variable %s declared at %s is never used in method %s.%s", name, loc, svc.Name, m.Name)
				}
			}
		}
	}
	return nil
}
