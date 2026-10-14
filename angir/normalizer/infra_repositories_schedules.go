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
		list, err := findersVal.List()
		if err != nil {
			return nil, fmt.Errorf("repository %s (%s): finders: %w", entity, formatPos(findersVal), err)
		}
		for idx := 0; list.Next(); idx++ {
			fv := list.Value()
			name, err := finderString(fv, "name")
			if err != nil {
				return nil, fmt.Errorf("repository %s, finder #%d (%s): name: %w", entity, idx+1, formatPos(fv), err)
			}
			if strings.TrimSpace(name) == "" {
				continue
			}
			// Every field that is present must decode: a value that fails
			// (a conflict, a wrong type) used to be read as "" and the finder
			// was generated without it.
			fieldErr := func(field string, err error) error {
				return fmt.Errorf("repository %s, finder %q (%s): %s: %w", entity, name, formatPos(fv), field, err)
			}
			str := func(field string) (string, error) {
				s, err := finderString(fv, field)
				if err != nil {
					return "", fieldErr(field, err)
				}
				return s, nil
			}
			action, err := str("action")
			if err != nil {
				return nil, err
			}
			returns, err := str("returns")
			if err != nil {
				return nil, err
			}
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
				whereIter, err := whereVal.List()
				if err != nil {
					return nil, fieldErr("where", err)
				}
				for whereIter.Next() {
					wv := whereIter.Value()
					var parts [4]string
					for i, key := range []string{"field", "op", "param", "param_type"} {
						s, err := finderString(wv, key)
						if err != nil {
							return nil, fieldErr("where."+key, err)
						}
						parts[i] = s
					}
					field, op, param, paramType := parts[0], parts[1], parts[2], parts[3]
					if strings.TrimSpace(field) == "" {
						continue
					}
					if strings.TrimSpace(param) == "" {
						param = field
					}
					explicit := strings.TrimSpace(paramType) != ""
					if !explicit {
						paramType = "string" // Default
					}
					if paramType == "time" {
						paramType = "time.Time"
					}
					wheres = append(wheres, FinderWhere{
						Field:             field,
						Op:                op,
						Param:             param,
						ParamType:         paramType,
						ParamTypeExplicit: explicit,
					})
				}
			}
			returnType, err := str("return_type")
			if err != nil {
				return nil, err
			}
			orderBy, err := str("order_by")
			if err != nil {
				return nil, err
			}
			customSQL, err := str("sql")
			if err != nil {
				return nil, err
			}
			sumField, err := str("sum_field")
			if err != nil {
				return nil, err
			}
			sumField = strings.TrimSpace(sumField)
			ext, err := extractFinderExtensions(fv, strings.TrimSpace(customSQL) != "", len(wheres) > 0, selectFields)
			if err != nil {
				return nil, fmt.Errorf("%s: repository %s, finder %q: %w", formatPos(fv), entity, name, err)
			}
			if ext.SelectAuto {
				selectFields = nil
			}
			result[entity] = append(result[entity], RepositoryFinder{
				Name:       name,
				Action:     action,
				Returns:    returns,
				ReturnType: strings.TrimSpace(returnType),
				Select:     selectFields,
				ScanFields: scanFields,
				Where:      wheres,
				OrderBy:    strings.TrimSpace(orderBy),
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
				CustomSQL:  strings.TrimSpace(customSQL),
				SumField:   sumField,
				Source:     formatPos(fv),
				Params:     ext.Params,
				Paged:      ext.Paged,
				Kind:       ext.Kind,
				OnConflict: ext.OnConflict,
				SelectAuto: ext.SelectAuto,
			})
		}
	}
	return result, nil
}

// FinderPaged reports whether a finder takes (limit int, offset int) after
// its other parameters: paged: true; or, with no paged: at all, the legacy
// rule — limit: > 0 or a name containing "paginate".
func FinderPaged(f RepositoryFinder) bool {
	if f.Paged != nil {
		return *f.Paged
	}
	return f.Limit > 0 || strings.Contains(strings.ToLower(f.Name), "paginate")
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
			Cron:    strings.TrimSpace(getString(v, "cron")),
			Source:  formatPos(v),
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
					continue
				}
				// The three-part form (publish:) has always dropped such a
				// value; a one-declaration schedule refuses it instead.
				if strings.TrimSpace(s.Publish) == "" {
					return nil, fmt.Errorf("%s: schedule %s: payload.%s must be an integer, a bool or a string", formatPos(fv), name, key)
				}
			}
		}
		schedules = append(schedules, s)
	}

	return schedules, nil
}
