package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

func (n *Normalizer) ExtractRepositories(val cue.Value) ([]Repository, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}
	var repos []Repository
	seen := make(map[string]bool)

	addRepo := func(ent string) {
		ent = strings.TrimSpace(ent)
		if ent == "" {
			return
		}
		repoName := ent + "Repository"
		if seen[repoName] {
			return
		}
		seen[repoName] = true
		repos = append(repos, Repository{
			Name:    repoName,
			Entity:  ent,
			Finders: nil,
		})
	}

	// 1. Extract from Services.{owns,entities}
	// `owns` is legacy, `entities` is the current architecture key.
	servicesVal := val.LookupPath(cue.ParsePath("Services"))
	if servicesVal.Exists() {
		iter, _ := servicesVal.Fields()
		for iter.Next() {
			svcVal := iter.Value()
			for _, field := range []string{"owns", "entities"} {
				entitiesVal := svcVal.LookupPath(cue.ParsePath(field))
				if !entitiesVal.Exists() {
					continue
				}
				list, _ := entitiesVal.List()
				for list.Next() {
					ent, _ := list.Value().String()
					addRepo(ent)
				}
			}
		}
	}

	// 2. Extract from Repositories (new style with finders)
	reposVal := val.LookupPath(cue.ParsePath("Repositories"))
	if reposVal.Exists() {
		iter, _ := reposVal.Fields()
		for iter.Next() {
			addRepo(iter.Selector().String())
		}
	}

	// 3. Extract from old-style labels (ends with Repository)
	iter, err := val.Fields(cue.All())
	if err == nil {
		for iter.Next() {
			label := iter.Selector().String()
			if !strings.HasSuffix(label, "Repository") || label == "Repositories" {
				continue
			}
			repoIter, _ := iter.Value().Fields(cue.All())
			for repoIter.Next() {
				addRepo(repoIter.Selector().String())
			}
		}
	}

	return repos, nil
}

// ExtractRepoFinders extracts finder definitions from cue/repo.
func (n *Normalizer) ExtractRepoFinders(val cue.Value) (map[string][]RepositoryFinder, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}
	reposVal := val.LookupPath(cue.ParsePath("Repositories"))
	if !reposVal.Exists() {
		return nil, nil
	}
	result := make(map[string][]RepositoryFinder)
	repoIter, err := reposVal.Fields(cue.All())
	if err != nil {
		return nil, err
	}
	for repoIter.Next() {
		entity := strings.TrimSpace(repoIter.Selector().String())
		if entity == "" {
			continue
		}
		repoVal := repoIter.Value()
		findersVal := repoVal.LookupPath(cue.ParsePath("finders"))
		if !findersVal.Exists() {
			continue
		}
		list, _ := findersVal.List()
		for list.Next() {
			fv := list.Value()
			name, _ := fv.LookupPath(cue.ParsePath("name")).String()
			if strings.TrimSpace(name) == "" {
				continue
			}
			action, _ := fv.LookupPath(cue.ParsePath("action")).String()
			returns, _ := fv.LookupPath(cue.ParsePath("returns")).String()
			var selectFields []string
			selVal := fv.LookupPath(cue.ParsePath("select"))
			if selVal.Exists() {
				if selVal.IncompleteKind() == cue.ListKind {
					selIter, _ := selVal.List()
					for selIter.Next() {
						s, _ := selIter.Value().String()
						if strings.TrimSpace(s) != "" {
							selectFields = append(selectFields, s)
						}
					}
				} else if s, err := selVal.String(); err == nil {
					selectFields = append(selectFields, s)
				}
			}
			var scanFields []string
			scanVal := fv.LookupPath(cue.ParsePath("scan_fields"))
			if scanVal.Exists() {
				if scanVal.IncompleteKind() == cue.ListKind {
					scanIter, _ := scanVal.List()
					for scanIter.Next() {
						s, _ := scanIter.Value().String()
						if strings.TrimSpace(s) != "" {
							scanFields = append(scanFields, s)
						}
					}
				} else if s, err := scanVal.String(); err == nil {
					scanFields = append(scanFields, s)
				}
			}
			var wheres []FinderWhere
			whereVal := fv.LookupPath(cue.ParsePath("where"))
			if whereVal.Exists() {
				whereIter, _ := whereVal.List()
				for whereIter.Next() {
					wv := whereIter.Value()
					field, _ := wv.LookupPath(cue.ParsePath("field")).String()
					op, _ := wv.LookupPath(cue.ParsePath("op")).String()
					param, _ := wv.LookupPath(cue.ParsePath("param")).String()
					paramType, _ := wv.LookupPath(cue.ParsePath("param_type")).String()
					if strings.TrimSpace(field) == "" {
						continue
					}
					if strings.TrimSpace(param) == "" {
						param = field
					}
					if strings.TrimSpace(paramType) == "" {
						paramType = "string" // Default
					}
					if paramType == "time" {
						paramType = "time.Time"
					}
					wheres = append(wheres, FinderWhere{
						Field:     field,
						Op:        op,
						Param:     param,
						ParamType: paramType,
					})
				}
			}
			returnType, _ := fv.LookupPath(cue.ParsePath("return_type")).String()
			if returnType != "" {
			}

			sumField := strings.TrimSpace(getString(fv, "sum_field"))
			result[entity] = append(result[entity], RepositoryFinder{
				Name:       name,
				Action:     action,
				Returns:    returns,
				ReturnType: strings.TrimSpace(returnType),
				Select:     selectFields,
				ScanFields: scanFields,
				Where:      wheres,
				OrderBy:    strings.TrimSpace(getString(fv, "order_by")),
				Limit: func() int {
					limitVal := fv.LookupPath(cue.ParsePath("limit"))
					if limitVal.Exists() {
						if v, err := limitVal.Int64(); err == nil && v > 0 {
							return int(v)
						}
					}
					return 0
				}(),
				ForUpdate: func() bool {
					val := fv.LookupPath(cue.ParsePath("for_update"))
					if val.Exists() {
						if v, err := val.Bool(); err == nil {
							return v
						}
					}
					return false
				}(),
				CustomSQL: strings.TrimSpace(getString(fv, "sql")),
				SumField:  sumField,
				Source:    formatPos(fv),
			})
		}
	}
	return result, nil
}

func (n *Normalizer) ExtractSchedules(val cue.Value) ([]ScheduleDef, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}
	var schedules []ScheduleDef

	sVal := val.LookupPath(cue.ParsePath("Schedules"))
	if !sVal.Exists() {
		return nil, nil
	}

	iter, _ := sVal.Fields()
	for iter.Next() {
		name := strings.TrimSpace(iter.Selector().String())
		v := iter.Value()
		s := ScheduleDef{
			Name:    name,
			Service: normalizeServiceName(getString(v, "service")),
			Action:  getString(v, "action"),
			At:      getString(v, "at"),
			Publish: getString(v, "publish"),
			Every:   getString(v, "every"),
		}
		payloadVal := v.LookupPath(cue.ParsePath("payload"))
		if payloadVal.Exists() {
			pit, _ := payloadVal.Fields(cue.All())
			for pit.Next() {
				key := strings.TrimSpace(pit.Selector().String())
				if key == "" {
					continue
				}
				fv := pit.Value()
				if i, err := fv.Int64(); err == nil {
					s.Payload = append(s.Payload, SchedulePayloadField{
						Name:  key,
						Type:  "int",
						Value: fmt.Sprint(i),
					})
					continue
				}
				if b, err := fv.Bool(); err == nil {
					if b {
						s.Payload = append(s.Payload, SchedulePayloadField{
							Name:  key,
							Type:  "bool",
							Value: "true",
						})
					} else {
						s.Payload = append(s.Payload, SchedulePayloadField{
							Name:  key,
							Type:  "bool",
							Value: "false",
						})
					}
					continue
				}
				if sVal, err := fv.String(); err == nil {
					s.Payload = append(s.Payload, SchedulePayloadField{
						Name:  key,
						Type:  "string",
						Value: sVal,
					})
				}
			}
		}
		schedules = append(schedules, s)
	}

	return schedules, nil
}
