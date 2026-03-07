package compiler

import (
	"fmt"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/flowsem"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
	"github.com/strogmv/ang/compiler/policy"
)

// ParsePhaseOutput is the explicit contract produced by the parse phase.
type ParsePhaseOutput struct {
	Domain      cue.Value
	Arch        cue.Value
	API         cue.Value
	Policy      cue.Value
	Repo        cue.Value
	Events      cue.Value
	Errors      cue.Value
	Project     cue.Value
	Projections cue.Value

	HasPolicy      bool
	HasRepo        bool
	HasProject     bool
	HasProjections bool
}

// NormalizePhaseOutput is the explicit contract produced by the normalize phase.
type NormalizePhaseOutput struct {
	Entities  []normalizer.Entity
	Services  []normalizer.Service
	Endpoints []normalizer.Endpoint
	Repos     []normalizer.Repository
	Events    []normalizer.EventDef
	Errors    []normalizer.ErrorDef
	Schedules []normalizer.ScheduleDef
	Scenarios []normalizer.ScenarioDef
	Scopes    []normalizer.ScopeDef
}

// FlowSemPhaseInput is the explicit contract accepted by the flow semantics phase.
type FlowSemPhaseInput struct {
	Services []normalizer.Service
}

// RunSemanticPhases runs parse -> normalize -> flowsem and returns typed semantic output.
func RunSemanticPhases(basePath string) (NormalizePhaseOutput, error) {
	return RunSemanticPhasesWithOptions(basePath, PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			LatestDiagnostics = append(LatestDiagnostics, w)
		},
	})
}

// RunSemanticPhasesWithOptions runs semantic phases with configurable diagnostics sink.
func RunSemanticPhasesWithOptions(basePath string, opts PipelineOptions) (NormalizePhaseOutput, error) {
	LatestDiagnostics = nil
	return runSemanticPhases(basePath, opts)
}

func runSemanticPhases(basePath string, opts PipelineOptions) (NormalizePhaseOutput, error) {
	parsed, err := runParsePhase(basePath, opts)
	if err != nil {
		return NormalizePhaseOutput{}, err
	}

	normalized, err := runNormalizePhase(parsed, opts)
	if err != nil {
		return NormalizePhaseOutput{}, err
	}

	runFlowSemPhase(FlowSemPhaseInput{
		Services: normalized.Services,
	}, opts)

	broadcastOnly, planned, compatAllowBreaking := loadEventAnnotations(basePath)
	emitEventUsageDiagnostics(normalized.Services, normalized.Events, normalized.Schedules, broadcastOnly, planned, opts)
	emitEventContractDiagnostics(basePath, normalized.Services, normalized.Events, compatAllowBreaking, opts)
	emitReadModelDiagnostics(normalized.Entities, normalized.Events, opts)

	return normalized, nil
}

func runParsePhase(basePath string, opts PipelineOptions) (ParsePhaseOutput, error) {
	out := ParsePhaseOutput{}
	p := parser.New()

	valDomain, _, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/domain"))
	if err != nil {
		return out, WrapContractError(
			StageCUE, ErrCodeCUEDomainLoad, "load cue/domain", fmt.Errorf("%s", parser.FormatCUELocationError(err)),
		)
	}
	valArch, _, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/architecture"))
	if err != nil {
		return out, WrapContractError(
			StageCUE, ErrCodeCUEArchLoad, "load cue/architecture", fmt.Errorf("%s", parser.FormatCUELocationError(err)),
		)
	}
	valAPI, _, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/api"))
	if err != nil {
		return out, WrapContractError(
			StageCUE, ErrCodeCUEAPILoad, "load cue/api", fmt.Errorf("%s", parser.FormatCUELocationError(err)),
		)
	}
	valPolicy, okPolicy, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/policy"))
	if !okPolicy {
		if legacyPolicy, legacyOK, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/policies")); legacyOK {
			valPolicy = legacyPolicy
			okPolicy = true
		}
	}
	valRepo, okRepo, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/repo"))
	valEvents, _, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/events"))
	valErrors, _, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/errors"))
	valProject, okProject, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/project"))
	valProjections, okProjections, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/projections"))

	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/domain"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/architecture"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/api"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/policy"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/policies"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/repo"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/events"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/errors"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/project"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/projections"), opts)

	out.Domain = valDomain
	out.Arch = valArch
	out.API = valAPI
	out.Policy = valPolicy
	out.Repo = valRepo
	out.Events = valEvents
	out.Errors = valErrors
	out.Project = valProject
	out.Projections = valProjections
	out.HasPolicy = okPolicy
	out.HasRepo = okRepo
	out.HasProject = okProject
	out.HasProjections = okProjections
	return out, nil
}

func runNormalizePhase(parsed ParsePhaseOutput, opts PipelineOptions) (NormalizePhaseOutput, error) {
	out := NormalizePhaseOutput{}

	n := normalizer.New()
	n.WarningSink = func(w normalizer.Warning) {
		LatestDiagnostics = append(LatestDiagnostics, w)
		if opts.WarningSink != nil {
			opts.WarningSink(w)
		}
	}

	archMode := strings.TrimSpace(opts.ArchitectureMode)
	allowCross := opts.AllowCrossService
	if parsed.HasProject && parsed.Project.Err() == nil {
		projectDef, err := n.ExtractProject(parsed.Project)
		if err != nil {
			return out, WrapContractError(StageCUE, ErrCodeCUEProjectParse, "extract project", err)
		}
		if projectDef != nil {
			if archMode == "" {
				archMode = strings.TrimSpace(projectDef.ArchitectureMode)
			}
			if len(allowCross) == 0 {
				allowCross = buildArchitectureAllowCrossMap(projectDef.AllowCrossService)
			}
		}
	}
	if archMode == "" {
		archMode = "strict"
	}
	n.ArchitectureMode = archMode
	if len(allowCross) > 0 {
		n.ArchitectureAllowCross = allowCross
	}

	entities, err := n.ExtractEntities(parsed.Domain)
	if err != nil {
		return out, WrapContractError(StageCUE, ErrCodeCUEEntityNormalize, "extract entities", err)
	}
	if parsed.HasProjections && parsed.Projections.Err() == nil {
		projectionEntities, err := n.ExtractEntities(parsed.Projections)
		if err != nil {
			return out, WrapContractError(StageCUE, ErrCodeCUEEntityNormalize, "extract projections", err)
		}
		if len(projectionEntities) > 0 {
			seenEntities := make(map[string]struct{}, len(entities))
			for _, e := range entities {
				seenEntities[e.Name] = struct{}{}
			}
			for _, pe := range projectionEntities {
				if _, exists := seenEntities[pe.Name]; exists {
					continue
				}
				if pe.Metadata == nil {
					pe.Metadata = make(map[string]any)
				}
				if _, ok := pe.Metadata["projection"]; !ok {
					pe.Metadata["projection"] = true
				}
				entities = append(entities, pe)
				seenEntities[pe.Name] = struct{}{}
			}
		}
	}

	emitFSMIntegrityDiagnostics(entities, opts)
	n.Policies = map[string]normalizer.PolicyDef{}
	if parsed.HasPolicy && parsed.Policy.Err() == nil {
		policies, err := n.ExtractPolicies(parsed.Policy)
		if err != nil {
			return out, WrapContractError(StageCUE, ErrCodeCUEPoliciesParse, "extract policies", err)
		}
		n.Policies = policies
	}
	services, err := n.ExtractServices(parsed.API, entities)
	if err != nil {
		return out, WrapContractError(StageCUE, ErrCodeCUEServiceNormalize, "extract services", err)
	}
	endpoints, err := n.ExtractEndpoints(parsed.API)
	if err != nil {
		return out, WrapContractError(StageCUE, ErrCodeCUEEndpointNormalize, "extract endpoints", err)
	}
	for _, ep := range endpoints {
		if err := policy.ValidateEndpoint(ep); err != nil {
			msg := fmt.Errorf("endpoint %s %s (%s): %w", ep.Method, ep.Path, ep.Source, err)
			return out, WrapContractError(StageCUE, ErrCodeCUEEndpointNormalize, "validate endpoint policy", msg)
		}
	}
	repos, err := n.ExtractRepositories(parsed.Arch)
	if err != nil {
		return out, WrapContractError(StageCUE, ErrCodeCUERepoNormalize, "extract repositories", err)
	}

	if parsed.HasRepo && parsed.Repo.Err() == nil {
		finderMap, _ := n.ExtractRepoFinders(parsed.Repo)
		if len(finderMap) > 0 {
			entityFieldMap := make(map[string]map[string]string)
			entityByName := make(map[string]normalizer.Entity)
			for _, e := range entities {
				fieldMap := make(map[string]string)
				for _, f := range e.Fields {
					fieldMap[strings.ToLower(f.Name)] = f.Type
				}
				entityFieldMap[e.Name] = fieldMap
				entityByName[e.Name] = e
			}
			projNameByKey := make(map[string]string)
			repoByEntity := make(map[string]int)
			for i := range repos {
				repoByEntity[repos[i].Entity] = i
			}
			for ent, finders := range finderMap {
				finders, projections := synthesizeImplicitProjections(ent, finders, entityByName, projNameByKey)
				for _, p := range projections {
					if _, ok := entityByName[p.Name]; ok {
						continue
					}
					entityByName[p.Name] = p
					entities = append(entities, p)
					fieldMap := make(map[string]string)
					for _, f := range p.Fields {
						fieldMap[strings.ToLower(f.Name)] = f.Type
					}
					entityFieldMap[p.Name] = fieldMap
				}
				emitSelectProjectionDiagnostics(ent, finders, entityFieldMap, opts)
				for fi := range finders {
					for wi := range finders[fi].Where {
						w := finders[fi].Where[wi]
						if (w.ParamType == "string" || w.ParamType == "") && entityFieldMap[ent] != nil {
							if t, ok := entityFieldMap[ent][strings.ToLower(w.Field)]; ok {
								finders[fi].Where[wi].ParamType = t
							}
						}
					}
				}
				if idx, ok := repoByEntity[ent]; ok {
					for _, f := range finders {
						seen := false
						for _, existing := range repos[idx].Finders {
							if strings.EqualFold(existing.Name, f.Name) {
								seen = true
								break
							}
						}
						if !seen {
							repos[idx].Finders = append(repos[idx].Finders, f)
						}
					}
					continue
				}
				repos = append(repos, normalizer.Repository{Name: ent + "Repository", Entity: ent, Finders: finders})
				repoByEntity[ent] = len(repos) - 1
			}
		}
	}

	var events []normalizer.EventDef
	if parsed.Events.Err() == nil {
		events, _ = n.ExtractEvents(parsed.Events)
	}
	if len(events) == 0 {
		archEvents, _ := n.ExtractEventsFromArch(parsed.Arch)
		events = append(events, archEvents...)
	}
	var bizErrors []normalizer.ErrorDef
	if parsed.Errors.Err() == nil {
		bizErrors, _ = n.ExtractErrors(parsed.Errors)
	}
	schedules, err := n.ExtractSchedules(parsed.API)
	if err != nil {
		return out, WrapContractError(StageCUE, ErrCodeCUEScheduleNormalize, "extract schedules", err)
	}

	scenarios, _ := n.ExtractScenarios(parsed.API)
	scopes, _ := n.ExtractScopes(parsed.Domain)

	out.Entities = entities
	out.Services = services
	out.Endpoints = endpoints
	out.Repos = repos
	out.Events = events
	out.Errors = bizErrors
	out.Schedules = schedules
	out.Scenarios = scenarios
	out.Scopes = scopes
	return out, nil
}

func runFlowSemPhase(in FlowSemPhaseInput, opts PipelineOptions) {
	for _, svc := range in.Services {
		for _, m := range svc.Methods {
			if len(m.Flow) == 0 {
				continue
			}
			issues := flowsem.Validate(toFlowSemSteps(m.Flow))
			for _, issue := range issues {
				sev := issue.Severity
				if sev == "" {
					sev = "error"
				}
				recordPipelineDiagnostic(normalizer.Warning{
					Kind:     "flow",
					Code:     issue.Code,
					Severity: sev,
					Message:  issue.Message,
					Op:       m.Name,
					Step:     issue.Step,
					Action:   issue.Action,
					File:     issue.File,
					Line:     issue.Line,
					Column:   issue.Column,
					CUEPath:  issue.CUEPath,
					Hint:     issue.Hint,
				}, opts)
			}
		}
	}
}

func toFlowSemSteps(steps []normalizer.FlowStep) []flowsem.Step {
	out := make([]flowsem.Step, 0, len(steps))
	for _, step := range steps {
		children := map[string][]flowsem.Step{}
		if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_do"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_ifNew"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_ifExists"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_then"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_else"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_default"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_catch"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_catch"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_fallback"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_fallback"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_onTimeout"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_onTimeout"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_onMissing"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_onMissing"] = toFlowSemSteps(v)
		}
		if v, ok := step.Args["_onMismatch"].([]normalizer.FlowStep); ok && len(v) > 0 {
			children["_onMismatch"] = toFlowSemSteps(v)
		}
		if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok && len(cases) > 0 {
			var merged []flowsem.Step
			for _, branch := range cases {
				merged = append(merged, toFlowSemSteps(branch)...)
			}
			if len(merged) > 0 {
				children["_cases"] = merged
			}
		}
		if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok && len(branches) > 0 {
			var merged []flowsem.Step
			for _, branch := range branches {
				merged = append(merged, toFlowSemSteps(branch)...)
			}
			if len(merged) > 0 {
				children["_branches"] = merged
			}
		}
		out = append(out, flowsem.Step{
			Action:   step.Action,
			Args:     step.Args,
			Children: children,
			File:     step.File,
			Line:     step.Line,
			Column:   step.Column,
			CUEPath:  step.CUEPath,
		})
	}
	return out
}

func buildArchitectureAllowCrossMap(rules []normalizer.CrossServiceRule) map[string]map[string]struct{} {
	if len(rules) == 0 {
		return nil
	}
	out := make(map[string]map[string]struct{})
	for _, rule := range rules {
		service := strings.TrimSpace(rule.Service)
		entity := strings.TrimSpace(rule.Entity)
		if service == "" || entity == "" {
			continue
		}
		service = strings.ToLower(service)
		entity = strings.ToLower(entity)
		if out[service] == nil {
			out[service] = make(map[string]struct{})
		}
		out[service][entity] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
