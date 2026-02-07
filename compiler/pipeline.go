package compiler

import (
	"fmt"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
	"github.com/strogmv/ang/compiler/transformers"
)

type PipelineOptions struct {
	WarningSink func(normalizer.Warning)
}

var LatestDiagnostics []normalizer.Warning

func RunPipeline(basePath string) ([]normalizer.Entity, []normalizer.Service, []normalizer.Endpoint, []normalizer.Repository, []normalizer.EventDef, []normalizer.ErrorDef, []normalizer.ScheduleDef, error) {
	return RunPipelineWithOptions(basePath, PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			LatestDiagnostics = append(LatestDiagnostics, w)
		},
	})
}

func RunPipelineWithOptions(basePath string, opts PipelineOptions) ([]normalizer.Entity, []normalizer.Service, []normalizer.Endpoint, []normalizer.Repository, []normalizer.EventDef, []normalizer.ErrorDef, []normalizer.ScheduleDef, error) {
	LatestDiagnostics = nil // Reset for new run
	p := parser.New()
	valDomain, _, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/domain"))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	valAPI, _, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/api"))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	valArch, _, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/architecture"))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	valRepo, okRepo, err := LoadOptionalDomain(p, filepath.Join(basePath, "cue/repo"))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	valEvents, _, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/events"))
	valErrors, _, _ := LoadOptionalDomain(p, filepath.Join(basePath, "cue/errors"))

	n := normalizer.New()
	n.WarningSink = func(w normalizer.Warning) {
		LatestDiagnostics = append(LatestDiagnostics, w)
		if opts.WarningSink != nil {
			opts.WarningSink(w)
		}
	}
	entities, err := n.ExtractEntities(valDomain)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	services, err := n.ExtractServices(valAPI, entities)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	endpoints, err := n.ExtractEndpoints(valAPI)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	repos, err := n.ExtractRepositories(valArch)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}

	if okRepo && valRepo.Err() == nil {
		finderMap, _ := n.ExtractRepoFinders(valRepo)
		if len(finderMap) > 0 {
			entityFieldMap := make(map[string]map[string]string)
			for _, e := range entities {
				fieldMap := make(map[string]string)
				for _, f := range e.Fields {
					fieldMap[strings.ToLower(f.Name)] = f.Type
				}
				entityFieldMap[e.Name] = fieldMap
			}
			repoByEntity := make(map[string]int)
			for i := range repos {
				repoByEntity[repos[i].Entity] = i
			}
			for ent, finders := range finderMap {
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
	var bizErrors []normalizer.ErrorDef
	if valErrors.Err() == nil {
		bizErrors, _ = n.ExtractErrors(valErrors)
	}
	schedules, err := n.ExtractSchedules(valAPI)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}

	return entities, services, endpoints, repos, events, bizErrors, schedules, nil
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

	registry := transformers.DefaultRegistry()
	if err := registry.Apply(schema); err != nil {
		return nil, fmt.Errorf("transformer error: %w", err)
	}

	hooks := transformers.DefaultHookRegistry()
	if err := hooks.Process(schema); err != nil {
		return nil, fmt.Errorf("hook error: %w", err)
	}

	return schema, nil
}
