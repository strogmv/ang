package angir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	cueparser "cuelang.org/go/cue/parser"
	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang-ir/parser"
)

type Result struct {
	Schema     *ir.Schema
	Normalized *Normalized
	Warnings   []normalizer.Warning
}

type Normalized struct {
	Entities       []normalizer.Entity
	Services       []normalizer.Service
	Endpoints      []normalizer.Endpoint
	Repos          []normalizer.Repository
	Events         []normalizer.EventDef
	Errors         []normalizer.ErrorDef
	Schedules      []normalizer.ScheduleDef
	Scenarios      []normalizer.ScenarioDef
	Scopes         []normalizer.ScopeDef
	Views          []normalizer.ViewDef
	Templates      []normalizer.TemplateDef
	EmailTemplates []normalizer.EmailTemplateDef
	Config         *normalizer.ConfigDef
	Auth           *normalizer.AuthDef
	RBAC           *normalizer.RBACDef
	Project        *normalizer.ProjectDef
}

func Load(basePath string) (*Result, error) {
	p := parser.New()
	n := normalizer.New()

	var warnings []normalizer.Warning
	n.WarningSink = func(w normalizer.Warning) {
		warnings = append(warnings, w)
	}

	valDomain, okDomain, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/domain"))
	if err != nil {
		return nil, fmt.Errorf("load cue/domain: %w", err)
	}
	valArch, _, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/architecture"))
	if err != nil {
		return nil, fmt.Errorf("load cue/architecture: %w", err)
	}
	valAPI, okAPI, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/api"))
	if err != nil {
		return nil, fmt.Errorf("load cue/api: %w", err)
	}
	legacyMainPath := filepath.Join(basePath, "cue", "main.cue")
	if !okDomain || !okAPI {
		if _, statErr := os.Stat(legacyMainPath); statErr == nil {
			legacyCue, legacyOK, legacyLoadErr := loadOptionalDomain(p, filepath.Join(basePath, "cue"))
			if legacyLoadErr != nil {
				return nil, fmt.Errorf("load legacy cue/: %w", legacyLoadErr)
			}
			if legacyOK {
				if !okDomain {
					valDomain = legacyCue
					okDomain = true
				}
				if !okAPI {
					valAPI = legacyCue
					okAPI = true
				}
			}
		}
	}

	valPolicy, okPolicy, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/policy"))
	if err != nil {
		return nil, fmt.Errorf("load cue/policy: %w", err)
	}
	if !okPolicy {
		if legacyPolicy, legacyOK, legacyErr := loadOptionalDomain(p, filepath.Join(basePath, "cue/policies")); legacyErr == nil && legacyOK {
			valPolicy = legacyPolicy
			okPolicy = true
		}
	}
	valRepo, okRepo, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/repo"))
	if err != nil {
		return nil, fmt.Errorf("load cue/repo: %w", err)
	}
	valEvents, okEvents, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/events"))
	if err != nil {
		return nil, fmt.Errorf("load cue/events: %w", err)
	}
	valErrors, okErrors, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/errors"))
	if err != nil {
		return nil, fmt.Errorf("load cue/errors: %w", err)
	}
	valProject, okProject, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/project"))
	if err != nil {
		return nil, fmt.Errorf("load cue/project: %w", err)
	}
	valViews, okViews, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/views"))
	if err != nil {
		return nil, fmt.Errorf("load cue/views: %w", err)
	}
	valInfra, okInfra, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/infra"))
	if err != nil {
		return nil, fmt.Errorf("load cue/infra: %w", err)
	}
	valEffects, okEffects, err := loadOptionalDomain(p, filepath.Join(basePath, "cue/effects"))
	if err != nil {
		return nil, fmt.Errorf("load cue/effects: %w", err)
	}

	entities, err := n.ExtractEntities(valDomain)
	if err != nil {
		return nil, fmt.Errorf("extract entities: %w", err)
	}
	services, err := n.ExtractServices(valAPI, entities)
	if err != nil {
		return nil, fmt.Errorf("extract services: %w", err)
	}
	endpoints, err := n.ExtractEndpoints(valAPI)
	if err != nil {
		return nil, fmt.Errorf("extract endpoints: %w", err)
	}
	repos, err := n.ExtractRepositories(valArch)
	if err != nil {
		return nil, fmt.Errorf("extract repositories: %w", err)
	}
	if okRepo && valRepo.Err() == nil {
		enrichRepoFinders(n, entities, &repos, valRepo)
	}

	events := []normalizer.EventDef{}
	if okEvents && valEvents.Err() == nil {
		events, err = n.ExtractEvents(valEvents)
		if err != nil {
			return nil, fmt.Errorf("extract events: %w", err)
		}
	}
	if len(events) == 0 {
		events, err = n.ExtractEventsFromArch(valArch)
		if err != nil {
			return nil, fmt.Errorf("extract events from architecture: %w", err)
		}
	}

	businessErrors := []normalizer.ErrorDef{}
	if okErrors && valErrors.Err() == nil {
		businessErrors, err = n.ExtractErrors(valErrors)
		if err != nil {
			return nil, fmt.Errorf("extract errors: %w", err)
		}
	}

	schedules, err := n.ExtractSchedules(valAPI)
	if err != nil {
		return nil, fmt.Errorf("extract schedules: %w", err)
	}
	scenarios, err := n.ExtractScenarios(valAPI)
	if err != nil {
		return nil, fmt.Errorf("extract scenarios: %w", err)
	}

	scopes := []normalizer.ScopeDef{}
	if okPolicy && valPolicy.Err() == nil {
		policies, err := n.ExtractPolicies(valPolicy)
		if err != nil {
			return nil, fmt.Errorf("extract policies: %w", err)
		}
		n.Policies = policies
		scopes, err = n.ExtractScopes(valPolicy)
		if err != nil {
			return nil, fmt.Errorf("extract scopes: %w", err)
		}
	}

	var views []normalizer.ViewDef
	if okViews && valViews.Err() == nil {
		views, err = n.ExtractViews(valViews)
		if err != nil {
			return nil, fmt.Errorf("extract views: %w", err)
		}
	}

	var projectDef *normalizer.ProjectDef
	if okProject && valProject.Err() == nil {
		projectDef, err = n.ExtractProject(valProject)
		if err != nil {
			return nil, fmt.Errorf("extract project: %w", err)
		}
	}

	var cfgDef *normalizer.ConfigDef
	var authDef *normalizer.AuthDef
	var sessionDef *normalizer.SessionDef
	var templates []normalizer.TemplateDef
	var emailTemplates []normalizer.EmailTemplateDef
	infraValues := map[string]any{}
	if okInfra && valInfra.Err() == nil {
		reg := normalizer.NewInfraRegistry()
		values, err := reg.ExtractAll(n, valInfra)
		if err != nil {
			return nil, fmt.Errorf("extract infra definitions: %w", err)
		}
		infraValues = mergeInfraValues(infraValues, values)
		cfgDef, err = n.ExtractConfig(valInfra)
		if err != nil {
			return nil, fmt.Errorf("extract config: %w", err)
		}
		authDef, err = n.ExtractAuth(valInfra)
		if err != nil {
			return nil, fmt.Errorf("extract auth: %w", err)
		}
		sessionDef, err = n.ExtractSession(valInfra)
		if err != nil {
			return nil, fmt.Errorf("extract session: %w", err)
		}
		_ = sessionDef
		templates, err = n.ExtractTemplates(valInfra)
		if err != nil {
			return nil, fmt.Errorf("extract templates: %w", err)
		}
		emailTemplates, err = n.ExtractEmailTemplates(valInfra)
		if err != nil {
			return nil, fmt.Errorf("extract email templates: %w", err)
		}
	}
	if okEffects && valEffects.Err() == nil {
		reg := normalizer.NewInfraRegistry()
		values, err := reg.ExtractAll(n, valEffects)
		if err != nil {
			return nil, fmt.Errorf("extract effect definitions: %w", err)
		}
		infraValues = mergeInfraValues(infraValues, values)
	}

	var rbacDef *normalizer.RBACDef
	if okPolicy && valPolicy.Err() == nil {
		rbacDef, err = n.ExtractRBAC(valPolicy)
		if err != nil {
			return nil, fmt.Errorf("extract rbac: %w", err)
		}
	}

	var cfgDefVal normalizer.ConfigDef
	if cfgDef != nil {
		cfgDefVal = *cfgDef
	}
	var projectDefVal normalizer.ProjectDef
	if projectDef != nil {
		projectDefVal = *projectDef
	}

	schema := ir.ConvertFromNormalizer(
		entities,
		services,
		events,
		businessErrors,
		endpoints,
		scopes,
		repos,
		cfgDefVal,
		authDef,
		rbacDef,
		schedules,
		views,
		projectDefVal,
	)
	schema.Templates = convertTemplates(templates, emailTemplates)
	schema.Notifications = convertNotifications(
		normalizer.InfraNotificationChannels(infraValues),
		normalizer.InfraNotificationPolicies(infraValues),
	)

	return &Result{
		Schema: schema,
		Normalized: &Normalized{
			Entities:       entities,
			Services:       services,
			Endpoints:      endpoints,
			Repos:          repos,
			Events:         events,
			Errors:         businessErrors,
			Schedules:      schedules,
			Scenarios:      scenarios,
			Scopes:         scopes,
			Views:          views,
			Templates:      templates,
			EmailTemplates: emailTemplates,
			Config:         cfgDef,
			Auth:           authDef,
			RBAC:           rbacDef,
			Project:        projectDef,
		},
		Warnings: warnings,
	}, nil
}

func LoadSchema(basePath string) (*ir.Schema, error) {
	result, err := Load(basePath)
	if err != nil {
		return nil, err
	}
	return result.Schema, nil
}

func enrichRepoFinders(n *normalizer.Normalizer, entities []normalizer.Entity, repos *[]normalizer.Repository, valRepo cue.Value) {
	finderMap, err := n.ExtractRepoFinders(valRepo)
	if err != nil || len(finderMap) == 0 {
		return
	}
	entityFieldMap := make(map[string]map[string]string)
	for _, e := range entities {
		fieldMap := make(map[string]string)
		for _, f := range e.Fields {
			fieldMap[strings.ToLower(f.Name)] = f.Type
		}
		entityFieldMap[e.Name] = fieldMap
	}
	repoByEntity := make(map[string]int)
	for i := range *repos {
		repoByEntity[(*repos)[i].Entity] = i
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
				for _, existing := range (*repos)[idx].Finders {
					if strings.EqualFold(existing.Name, f.Name) {
						seen = true
						break
					}
				}
				if !seen {
					(*repos)[idx].Finders = append((*repos)[idx].Finders, f)
				}
			}
			continue
		}
		*repos = append(*repos, normalizer.Repository{Name: ent + "Repository", Entity: ent, Finders: finders})
		repoByEntity[ent] = len(*repos) - 1
	}
}

func convertTemplates(templates []normalizer.TemplateDef, emailTemplates []normalizer.EmailTemplateDef) []ir.Template {
	out := make([]ir.Template, 0, len(templates)+len(emailTemplates))
	for _, tpl := range templates {
		out = append(out, ir.Template{
			ID:           tpl.ID,
			Kind:         tpl.Kind,
			Channel:      tpl.Channel,
			Locale:       tpl.Locale,
			Version:      tpl.Version,
			Engine:       tpl.Engine,
			Subject:      tpl.Subject,
			Text:         tpl.Text,
			HTML:         tpl.HTML,
			Body:         tpl.Body,
			RequiredVars: append([]string(nil), tpl.RequiredVars...),
			OptionalVars: append([]string(nil), tpl.OptionalVars...),
		})
	}
	for _, tpl := range emailTemplates {
		out = append(out, ir.Template{
			ID:      tpl.Name,
			Kind:    "email",
			Channel: "email",
			Subject: tpl.Subject,
			Text:    tpl.Text,
			HTML:    tpl.HTML,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func convertNotifications(ch *normalizer.NotificationChannelsDef, pol *normalizer.NotificationPoliciesDef) *ir.NotificationsConfig {
	if ch == nil && pol == nil {
		return nil
	}
	out := &ir.NotificationsConfig{}
	if ch != nil {
		nc := &ir.NotificationChannels{
			Enabled:         ch.Enabled,
			DefaultChannels: append([]string(nil), ch.DefaultChannels...),
		}
		if len(ch.Channels) > 0 {
			nc.Channels = make(map[string]ir.NotificationChannelSpec, len(ch.Channels))
			for name, cfg := range ch.Channels {
				nc.Channels[name] = ir.NotificationChannelSpec{
					Enabled:    cfg.Enabled,
					Driver:     cfg.Driver,
					Topic:      cfg.Topic,
					Subject:    cfg.Subject,
					Template:   cfg.Template,
					DSNEnv:     cfg.DSNEnv,
					BrokersEnv: cfg.BrokersEnv,
				}
			}
		}
		out.Channels = nc
	}
	if pol != nil {
		np := &ir.NotificationPolicies{Enabled: pol.Enabled}
		for _, rule := range pol.Rules {
			np.Rules = append(np.Rules, ir.NotificationPolicyRule{
				Enabled:  rule.Enabled,
				Event:    rule.Event,
				Type:     rule.Type,
				Audience: rule.Audience,
				Channels: append([]string(nil), rule.Channels...),
				Template: rule.Template,
				MuteKey:  rule.MuteKey,
			})
		}
		out.Policies = np
	}
	return out
}

func loadOptionalDomain(p *parser.Parser, path string) (cue.Value, bool, error) {
	matches, _ := filepath.Glob(filepath.Join(path, "*.cue"))
	if len(matches) == 0 {
		return cue.Value{}, false, nil
	}
	val, err := p.LoadDomain(path)
	if err != nil {
		filtered, skipped, filterErr := filterValidCUEFiles(matches)
		if filterErr != nil || len(skipped) == 0 || len(filtered) == 0 {
			return cue.Value{}, false, err
		}
		tmpDir, mkErr := os.MkdirTemp("", "ang-ir-partial-cue-*")
		if mkErr != nil {
			return cue.Value{}, false, err
		}
		defer os.RemoveAll(tmpDir)
		for _, src := range filtered {
			data, readErr := os.ReadFile(src)
			if readErr != nil {
				return cue.Value{}, false, err
			}
			dst := filepath.Join(tmpDir, filepath.Base(src))
			if writeErr := os.WriteFile(dst, data, 0o644); writeErr != nil {
				return cue.Value{}, false, err
			}
		}
		val2, err2 := p.LoadDomain(tmpDir)
		if err2 != nil {
			return cue.Value{}, false, err
		}
		return val2, true, nil
	}
	return val, true, nil
}

func filterValidCUEFiles(files []string) (valid []string, skipped map[string]error, err error) {
	valid = make([]string, 0, len(files))
	skipped = make(map[string]error)
	for _, file := range files {
		src, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, nil, readErr
		}
		if _, parseErr := cueparser.ParseFile(file, src, cueparser.ParseComments); parseErr != nil {
			skipped[file] = parseErr
			continue
		}
		valid = append(valid, file)
	}
	return valid, skipped, nil
}

func mergeInfraValues(base map[string]any, overlay map[string]any) map[string]any {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		switch k {
		case normalizer.InfraKeyEffectHandlers, normalizer.InfraKeyEffectTestHandlers:
			merged := &normalizer.EffectHandlersDef{Bindings: map[string]normalizer.EffectHandlerBinding{}}
			if existing, ok := out[k].(*normalizer.EffectHandlersDef); ok && existing != nil {
				for kind, binding := range existing.Bindings {
					merged.Bindings[kind] = binding
				}
			}
			if incoming, ok := v.(*normalizer.EffectHandlersDef); ok && incoming != nil {
				for kind, binding := range incoming.Bindings {
					merged.Bindings[kind] = binding
				}
			}
			out[k] = merged
		case normalizer.InfraKeyEffectMiddleware:
			merged := &normalizer.EffectMiddlewareCatalogDef{Chains: map[string][]normalizer.EffectMiddlewareDef{}}
			if existing, ok := out[k].(*normalizer.EffectMiddlewareCatalogDef); ok && existing != nil {
				for kind, chain := range existing.Chains {
					merged.Chains[kind] = append([]normalizer.EffectMiddlewareDef(nil), chain...)
				}
			}
			if incoming, ok := v.(*normalizer.EffectMiddlewareCatalogDef); ok && incoming != nil {
				for kind, chain := range incoming.Chains {
					merged.Chains[kind] = append([]normalizer.EffectMiddlewareDef(nil), chain...)
				}
			}
			out[k] = merged
		default:
			out[k] = v
		}
	}
	return out
}
