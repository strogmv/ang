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
	"github.com/strogmv/ang/compiler/transformers"
)

const (
	Version       = "0.1.80"
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
	WarningSink func(normalizer.Warning)
}

var LatestDiagnostics []normalizer.Warning

func RunPipeline(basePath string) ([]normalizer.Entity, []normalizer.Service, []normalizer.Endpoint, []normalizer.Repository, []normalizer.EventDef, []normalizer.ErrorDef, []normalizer.ScheduleDef, []normalizer.ScenarioDef, error) {
	return RunPipelineWithOptions(basePath, PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			LatestDiagnostics = append(LatestDiagnostics, w)
		},
	})
}

func RunPipelineWithOptions(basePath string, opts PipelineOptions) ([]normalizer.Entity, []normalizer.Service, []normalizer.Endpoint, []normalizer.Repository, []normalizer.EventDef, []normalizer.ErrorDef, []normalizer.ScheduleDef, []normalizer.ScenarioDef, error) {
	LatestDiagnostics = nil // Reset for new run
	p := parser.New()

	valDomain, _, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/domain"))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, WrapContractError(
			StageCUE, ErrCodeCUEDomainLoad, "load cue/domain", fmt.Errorf("%s", parser.FormatCUELocationError(err)),
		)
	}
	valArch, _, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/architecture"))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, WrapContractError(
			StageCUE, ErrCodeCUEArchLoad, "load cue/architecture", fmt.Errorf("%s", parser.FormatCUELocationError(err)),
		)
	}
	valAPI, _, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/api"))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, WrapContractError(
			StageCUE, ErrCodeCUEAPILoad, "load cue/api", fmt.Errorf("%s", parser.FormatCUELocationError(err)),
		)
	}
	valRepo, okRepo, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/repo"))
	valEvents, _, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/events"))
	valErrors, _, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/errors"))

	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/domain"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/architecture"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/api"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/repo"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/events"), opts)
	emitFileSizeDiagnostics(filepath.Join(basePath, "cue/errors"), opts)

	n := normalizer.New()
	n.WarningSink = func(w normalizer.Warning) {
		LatestDiagnostics = append(LatestDiagnostics, w)
		if opts.WarningSink != nil {
			opts.WarningSink(w)
		}
	}
	entities, err := n.ExtractEntities(valDomain)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, WrapContractError(StageCUE, ErrCodeCUEEntityNormalize, "extract entities", err)
	}
	emitFSMIntegrityDiagnostics(entities, opts)
	services, err := n.ExtractServices(valAPI, entities)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, WrapContractError(StageCUE, ErrCodeCUEServiceNormalize, "extract services", err)
	}
	endpoints, err := n.ExtractEndpoints(valAPI)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, WrapContractError(StageCUE, ErrCodeCUEEndpointNormalize, "extract endpoints", err)
	}
	repos, err := n.ExtractRepositories(valArch)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, WrapContractError(StageCUE, ErrCodeCUERepoNormalize, "extract repositories", err)
	}

	if okRepo && valRepo.Err() == nil {
		finderMap, _ := n.ExtractRepoFinders(valRepo)
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
	if valEvents.Err() == nil {
		events, _ = n.ExtractEvents(valEvents)
	}
	if len(events) == 0 {
		archEvents, _ := n.ExtractEventsFromArch(valArch)
		events = append(events, archEvents...)
	}
	var bizErrors []normalizer.ErrorDef
	if valErrors.Err() == nil {
		bizErrors, _ = n.ExtractErrors(valErrors)
	}
	schedules, err := n.ExtractSchedules(valAPI)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, WrapContractError(StageCUE, ErrCodeCUEScheduleNormalize, "extract schedules", err)
	}

	scenarios, _ := n.ExtractScenarios(valAPI)

	return entities, services, endpoints, repos, events, bizErrors, schedules, scenarios, nil
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
	errors []normalizer.ErrorDef, endpoints []normalizer.Endpoint, repos []normalizer.Repository,
	config normalizer.ConfigDef, auth *normalizer.AuthDef, rbac *normalizer.RBACDef,
	schedules []normalizer.ScheduleDef, views []normalizer.ViewDef, project normalizer.ProjectDef,
) (*ir.Schema, error) {
	schema := ir.ConvertFromNormalizer(entities, services, events, errors, endpoints, repos, config, auth, rbac, schedules, views, project)
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
