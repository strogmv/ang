package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

// LoadCodegenConfig загружает маппинг типов и настройки фич из CUE.
func (n *Normalizer) LoadCodegenConfig(val cue.Value) error {
	codegen := val.LookupPath(cue.ParsePath("#Codegen"))
	if !codegen.Exists() {
		return nil
	}

	mappingVal := codegen.LookupPath(cue.ParsePath("type_mapping"))
	if mappingVal.Exists() {
		iter, _ := mappingVal.Fields()
		for iter.Next() {
			typeName := iter.Selector().String()
			v := iter.Value()

			cfg := TypeConfig{
				GoType:     getString(v, "type"),
				Package:    getString(v, "pkg"),
				SQLType:    getString(v, "sql"),
				NullHelper: getString(v, "null"),
			}
			n.TypeMapping[typeName] = cfg
		}
	}
	return nil
}

// ExtractConfig parses #AppConfig.
func (n *Normalizer) ExtractConfig(val cue.Value) (*ConfigDef, error) {
	cfgVal := val.LookupPath(cue.ParsePath("#AppConfig"))
	if !cfgVal.Exists() {
		return nil, nil
	}

	ent, err := n.parseEntity("AppConfig", cfgVal)
	if err != nil {
		return nil, err
	}
	return &ConfigDef{Fields: ent.Fields}, nil
}

// ExtractAuth parses #Auth definition from infra.
func (n *Normalizer) ExtractAuth(val cue.Value) (*AuthDef, error) {
	authVal := val.LookupPath(cue.ParsePath("#Auth"))
	if !authVal.Exists() {
		return nil, nil
	}

	jwtVal := authVal.LookupPath(cue.ParsePath("jwt"))
	if !jwtVal.Exists() {
		return nil, nil
	}

	getClaim := func(name, def string) string {
		v := jwtVal.LookupPath(cue.ParsePath("claims." + name + ".field"))
		s, _ := v.String()
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		return s
	}

	alg, _ := jwtVal.LookupPath(cue.ParsePath("alg")).String()
	alg = strings.TrimSpace(alg)
	issuer, _ := jwtVal.LookupPath(cue.ParsePath("issuer")).String()
	issuer = strings.TrimSpace(issuer)
	audience, _ := jwtVal.LookupPath(cue.ParsePath("audience")).String()
	audience = strings.TrimSpace(audience)
	accessTTL, _ := jwtVal.LookupPath(cue.ParsePath("tokens.access_ttl")).String()
	refreshTTL, _ := jwtVal.LookupPath(cue.ParsePath("tokens.refresh_ttl")).String()
	rotation, _ := jwtVal.LookupPath(cue.ParsePath("tokens.rotation")).Bool()
	refreshStore, _ := jwtVal.LookupPath(cue.ParsePath("tokens.store")).String()
	if alg == "" {
		alg = "RS256"
	}

	service, _ := authVal.LookupPath(cue.ParsePath("service")).String()
	service = normalizeServiceName(strings.TrimSpace(service))
	loginOp, _ := jwtVal.LookupPath(cue.ParsePath("ops.login.op")).String()
	loginAccess, _ := jwtVal.LookupPath(cue.ParsePath("ops.login.access_field")).String()
	loginRefresh, _ := jwtVal.LookupPath(cue.ParsePath("ops.login.refresh_field")).String()
	refreshOp, _ := jwtVal.LookupPath(cue.ParsePath("ops.refresh.op")).String()
	refreshTokenField, _ := jwtVal.LookupPath(cue.ParsePath("ops.refresh.token_field")).String()
	refreshAccess, _ := jwtVal.LookupPath(cue.ParsePath("ops.refresh.access_field")).String()
	refreshRefresh, _ := jwtVal.LookupPath(cue.ParsePath("ops.refresh.refresh_field")).String()
	logoutOp, _ := jwtVal.LookupPath(cue.ParsePath("ops.logout.op")).String()
	logoutTokenField, _ := jwtVal.LookupPath(cue.ParsePath("ops.logout.token_field")).String()

	return &AuthDef{
		Alg:                 alg,
		Issuer:              issuer,
		Audience:            audience,
		UserIDClaim:         getClaim("userId", "sub"),
		CompanyIDClaim:      getClaim("companyId", "cid"),
		RolesClaim:          getClaim("roles", "roles"),
		PermissionsClaim:    getClaim("perms", "perms"),
		AccessTTL:           strings.TrimSpace(accessTTL),
		RefreshTTL:          strings.TrimSpace(refreshTTL),
		Rotation:            rotation,
		RefreshStore:        strings.TrimSpace(refreshStore),
		Service:             service,
		LoginOp:             strings.TrimSpace(loginOp),
		LoginAccessField:    strings.TrimSpace(loginAccess),
		LoginRefreshField:   strings.TrimSpace(loginRefresh),
		RefreshOp:           strings.TrimSpace(refreshOp),
		RefreshTokenField:   strings.TrimSpace(refreshTokenField),
		RefreshAccessField:  strings.TrimSpace(refreshAccess),
		RefreshRefreshField: strings.TrimSpace(refreshRefresh),
		LogoutOp:            strings.TrimSpace(logoutOp),
		LogoutTokenField:    strings.TrimSpace(logoutTokenField),
	}, nil
}

// ExtractRBAC parses #RBAC.
func (n *Normalizer) ExtractRBAC(val cue.Value) (*RBACDef, error) {
	rbacVal := val.LookupPath(cue.ParsePath("#RBAC"))
	if !rbacVal.Exists() {
		return n.extractRBACFromPolicy(val)
	}

	rbac := &RBACDef{
		Roles:       make(map[string][]string),
		Permissions: make(map[string]string),
	}

	rolesVal := rbacVal.LookupPath(cue.ParsePath("roles"))
	iter, _ := rolesVal.Fields()
	for iter.Next() {
		roleName := iter.Selector().String()
		var perms []string
		list, _ := iter.Value().List()
		for list.Next() {
			p, _ := list.Value().String()
			perms = append(perms, strings.Trim(p, ""))
		}
		rbac.Roles[roleName] = perms
	}

	permsVal := rbacVal.LookupPath(cue.ParsePath("permissions"))
	if permsVal.Exists() {
		pit, _ := permsVal.Fields()
		for pit.Next() {
			permName := strings.TrimSpace(pit.Selector().String())
			desc, _ := pit.Value().String()
			rbac.Permissions[permName] = strings.TrimSpace(desc)
		}
	}

	return rbac, nil
}

func (n *Normalizer) extractRBACFromPolicy(val cue.Value) (*RBACDef, error) {
	rolesVal := val.LookupPath(cue.ParsePath("Roles"))
	actionsVal := val.LookupPath(cue.ParsePath("Actions"))
	policiesVal := val.LookupPath(cue.ParsePath("Policies"))
	if !rolesVal.Exists() && !actionsVal.Exists() && !policiesVal.Exists() {
		return nil, nil
	}

	rbac := &RBACDef{
		Roles:       make(map[string][]string),
		Permissions: make(map[string]string),
	}

	allPerms := make(map[string]bool)
	actionByResource := make(map[string][]string)

	if actionsVal.Exists() {
		rIter, _ := actionsVal.Fields()
		for rIter.Next() {
			resource := strings.TrimSpace(rIter.Selector().String())
			aIter, _ := rIter.Value().Fields()
			for aIter.Next() {
				action := strings.TrimSpace(aIter.Selector().String())
				perm := resource + "." + action
				allPerms[perm] = true
				actionByResource[resource] = append(actionByResource[resource], perm)
			}
		}
	}

	if policiesVal.Exists() {
		pIter, _ := policiesVal.Fields()
		for pIter.Next() {
			roleName := strings.TrimSpace(pIter.Selector().String())
			allowVal := pIter.Value().LookupPath(cue.ParsePath("allow"))
			if !allowVal.Exists() {
				continue
			}
			list, _ := allowVal.List()
			for list.Next() {
				raw, _ := list.Value().String()
				pattern := strings.TrimSpace(raw)
				if pattern == "*" {
					for perm := range allPerms {
						rbac.Roles[roleName] = append(rbac.Roles[roleName], perm)
					}
					continue
				}
				if strings.HasSuffix(pattern, ".*") {
					resource := strings.TrimSuffix(pattern, ".*")
					for _, perm := range actionByResource[resource] {
						rbac.Roles[roleName] = append(rbac.Roles[roleName], perm)
					}
					continue
				}
				rbac.Roles[roleName] = append(rbac.Roles[roleName], pattern)
				allPerms[pattern] = true
			}
		}
	}

	for perm := range allPerms {
		rbac.Permissions[perm] = ""
	}

	return rbac, nil
}

// ExtractRepositories extracts repository definitions.
func (n *Normalizer) ExtractRepositories(val cue.Value) ([]Repository, error) {
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

	// 1. Extract from Services.owns
	servicesVal := val.LookupPath(cue.ParsePath("Services"))
	if servicesVal.Exists() {
		iter, _ := servicesVal.Fields()
		for iter.Next() {
			svcVal := iter.Value()
			ownsVal := svcVal.LookupPath(cue.ParsePath("owns"))
			if ownsVal.Exists() {
				list, _ := ownsVal.List()
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
				fmt.Printf("DEBUG infra: Entity=%s Name=%s ReturnType=%s\n", entity, name, returnType)
			}
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
			})
		}
	}
	return result, nil
}

func (n *Normalizer) ExtractSchedules(val cue.Value) ([]ScheduleDef, error) {
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

func (n *Normalizer) ExtractProject(val cue.Value) (*ProjectDef, error) {
	projectVal := val.LookupPath(cue.ParsePath("#Project"))
	if !projectVal.Exists() {
		return nil, nil
	}
	name := strings.TrimSpace(getString(projectVal, "name"))
	version := strings.TrimSpace(getString(projectVal, "version"))
	if name == "" && version == "" {
		return nil, nil
	}
	return &ProjectDef{
		Name:    name,
		Version: version,
	}, nil
}

// ExtractTarget parses #Target from project.cue.
func (n *Normalizer) ExtractTarget(val cue.Value) (*TargetDef, error) {
	targetVal := val.LookupPath(cue.ParsePath("#Target"))
	if !targetVal.Exists() {
		// Return defaults
		return &TargetDef{
			Lang:      "go",
			Framework: "chi",
			DB:        "postgres",
			Cache:     "redis",
			Queue:     "nats",
			Storage:   "s3",
		}, nil
	}

	return &TargetDef{
		Lang:      getStringWithDefault(targetVal, "lang", "go"),
		Framework: getStringWithDefault(targetVal, "framework", "chi"),
		DB:        getStringWithDefault(targetVal, "db", "postgres"),
		Cache:     getStringWithDefault(targetVal, "cache", "redis"),
		Queue:     getStringWithDefault(targetVal, "queue", "nats"),
		Storage:   getStringWithDefault(targetVal, "storage", "s3"),
	}, nil
}

// ExtractTransformersConfig parses #Transformers from project.cue.
func (n *Normalizer) ExtractTransformersConfig(val cue.Value) (*TransformersConfig, error) {
	trVal := val.LookupPath(cue.ParsePath("#Transformers"))
	if !trVal.Exists() {
		// Return defaults
		return &TransformersConfig{
			Timestamps:  true,
			SoftDelete:  false,
			Image:       true,
			ThumbSuffix: "_thumb",
			Validation:  true,
		}, nil
	}

	cfg := &TransformersConfig{
		Timestamps:  true,
		SoftDelete:  false,
		Image:       true,
		ThumbSuffix: "_thumb",
		Validation:  true,
	}

	// Parse timestamps
	if ts := trVal.LookupPath(cue.ParsePath("timestamps")); ts.Exists() {
		if enabled := ts.LookupPath(cue.ParsePath("enabled")); enabled.Exists() {
			if b, err := enabled.Bool(); err == nil {
				cfg.Timestamps = b
			}
		}
	}

	// Parse soft_delete
	if sd := trVal.LookupPath(cue.ParsePath("soft_delete")); sd.Exists() {
		if enabled := sd.LookupPath(cue.ParsePath("enabled")); enabled.Exists() {
			if b, err := enabled.Bool(); err == nil {
				cfg.SoftDelete = b
			}
		}
	}

	// Parse image
	if img := trVal.LookupPath(cue.ParsePath("image")); img.Exists() {
		if enabled := img.LookupPath(cue.ParsePath("enabled")); enabled.Exists() {
			if b, err := enabled.Bool(); err == nil {
				cfg.Image = b
			}
		}
		if suffix := img.LookupPath(cue.ParsePath("thumb_suffix")); suffix.Exists() {
			if s, err := suffix.String(); err == nil {
				cfg.ThumbSuffix = s
			}
		}
	}

	// Parse validation
	if vl := trVal.LookupPath(cue.ParsePath("validation")); vl.Exists() {
		if enabled := vl.LookupPath(cue.ParsePath("enabled")); enabled.Exists() {
			if b, err := enabled.Bool(); err == nil {
				cfg.Validation = b
			}
		}
	}

	return cfg, nil
}

func getStringWithDefault(v cue.Value, path, def string) string {
	res := v.LookupPath(cue.ParsePath(path))
	if !res.Exists() {
		return def
	}
	s, err := res.String()
	if err != nil {
		return def
	}
	return strings.TrimSpace(s)
}
