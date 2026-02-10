package ir_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
	"github.com/strogmv/ang/compiler/transformers"
)

type irSnapshot struct {
	IRVersion string             `json:"ir_version"`
	Entities  []string           `json:"entities"`
	Services  []serviceSnapshot  `json:"services"`
	Endpoints []endpointSnapshot `json:"endpoints"`
	Repos     []repoSnapshot     `json:"repos"`
	Events    []eventSnapshot    `json:"events"`
	Errors    []errorSnapshot    `json:"errors"`
}

type serviceSnapshot struct {
	Name      string   `json:"name"`
	Methods   []string `json:"methods"`
	Uses      []string `json:"uses"`
	Publishes []string `json:"publishes"`
}

type endpointSnapshot struct {
	RPC            string   `json:"rpc"`
	Method         string   `json:"method"`
	Path           string   `json:"path"`
	Service        string   `json:"service"`
	AuthType       string   `json:"auth_type,omitempty"`
	AuthRoles      []string `json:"auth_roles,omitempty"`
	PaginationType string   `json:"pagination_type,omitempty"`
}

type repoSnapshot struct {
	Name    string   `json:"name"`
	Entity  string   `json:"entity"`
	Finders []string `json:"finders"`
}

type eventSnapshot struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

type errorSnapshot struct {
	Name       string `json:"name"`
	Code       int    `json:"code"`
	HTTPStatus int    `json:"http_status"`
}

func TestIRSnapshots(t *testing.T) {
	before, after := buildIRSnapshots(t)

	root := repoRoot(t)
	snapDir := filepath.Join(root, "tests", "snapshots")
	beforePath := filepath.Join(snapDir, "ir.before.json")
	afterPath := filepath.Join(snapDir, "ir.after.json")

	update := os.Getenv("UPDATE_IR_SNAPSHOTS") == "1"
	if update {
		writeSnapshot(t, beforePath, before)
		writeSnapshot(t, afterPath, after)
		return
	}

	assertSnapshot(t, beforePath, before)
	assertSnapshot(t, afterPath, after)
}

func buildIRSnapshots(t *testing.T) (irSnapshot, irSnapshot) {
	t.Helper()

	p := parser.New()
	n := normalizer.New()

	root := repoRoot(t)

	valDomain := mustLoadDomain(t, p, filepath.Join(root, "cue", "domain"))
	valAPI := mustLoadDomain(t, p, filepath.Join(root, "cue", "api"))
	valArch := mustLoadDomain(t, p, filepath.Join(root, "cue", "architecture"))

	valRepo, okRepo, err := loadOptionalDomain(t, p, filepath.Join(root, "cue", "repo"))
	if err != nil {
		t.Fatalf("load repo domain: %v", err)
	}
	valEvents, okEvents, err := loadOptionalDomain(t, p, filepath.Join(root, "cue", "events"))
	if err != nil {
		t.Fatalf("load events domain: %v", err)
	}
	valErrors, okErrors, err := loadOptionalDomain(t, p, filepath.Join(root, "cue", "errors"))
	if err != nil {
		t.Fatalf("load errors domain: %v", err)
	}

	var cfgDef *normalizer.ConfigDef
	var authDef *normalizer.AuthDef
	if val, ok := mustLoadOptionalDomain(t, p, filepath.Join(root, "cue", "infra")); ok {
		var err error
		cfgDef, err = n.ExtractConfig(val)
		if err != nil {
			t.Fatalf("extract config: %v", err)
		}
		authDef, err = n.ExtractAuth(val)
		if err != nil {
			t.Fatalf("extract auth: %v", err)
		}
	}

	var rbacDef *normalizer.RBACDef
	if val, ok := mustLoadOptionalDomain(t, p, filepath.Join(root, "cue", "rbac")); ok {
		var err error
		rbacDef, err = n.ExtractRBAC(val)
		if err != nil {
			t.Fatalf("extract rbac: %v", err)
		}
	} else if val, ok := mustLoadOptionalDomain(t, p, filepath.Join(root, "cue", "policies")); ok {
		var err error
		rbacDef, err = n.ExtractRBAC(val)
		if err != nil {
			t.Fatalf("extract rbac (policies): %v", err)
		}
	}

	var views []normalizer.ViewDef
	if val, ok := mustLoadOptionalDomain(t, p, filepath.Join(root, "cue", "views")); ok {
		var err error
		views, err = n.ExtractViews(val)
		if err != nil {
			t.Fatalf("extract views: %v", err)
		}
	}

	var projectDef *normalizer.ProjectDef
	if val, ok := mustLoadOptionalDomain(t, p, filepath.Join(root, "cue", "project")); ok {
		var err error
		projectDef, err = n.ExtractProject(val)
		if err != nil {
			t.Fatalf("extract project: %v", err)
		}
	}

	entities, err := n.ExtractEntities(valDomain)
	if err != nil {
		t.Fatalf("extract entities: %v", err)
	}
	services, err := n.ExtractServices(valAPI, entities)
	if err != nil {
		t.Fatalf("extract services: %v", err)
	}
	endpoints, err := n.ExtractEndpoints(valAPI)
	if err != nil {
		t.Fatalf("extract endpoints: %v", err)
	}
	repos, err := n.ExtractRepositories(valArch)
	if err != nil {
		t.Fatalf("extract repos: %v", err)
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
	if okEvents && valEvents.Err() == nil {
		events, _ = n.ExtractEvents(valEvents)
	}
	var bizErrors []normalizer.ErrorDef
	if okErrors && valErrors.Err() == nil {
		bizErrors, _ = n.ExtractErrors(valErrors)
	}
	schedules, err := n.ExtractSchedules(valAPI)
	if err != nil {
		t.Fatalf("extract schedules: %v", err)
	}

	var cfgDefVal normalizer.ConfigDef
	if cfgDef != nil {
		cfgDefVal = *cfgDef
	}
	var projectDefVal normalizer.ProjectDef
	if projectDef != nil {
		projectDefVal = *projectDef
	}

	schemaBefore := ir.ConvertFromNormalizer(
		entities, services, events, bizErrors, endpoints, repos,
		cfgDefVal, authDef, rbacDef, schedules, views, projectDefVal,
	)

	schemaAfter := ir.ConvertFromNormalizer(
		entities, services, events, bizErrors, endpoints, repos,
		cfgDefVal, authDef, rbacDef, schedules, views, projectDefVal,
	)

	registry := transformers.DefaultRegistry()
	if err := registry.Apply(schemaAfter); err != nil {
		t.Fatalf("transformers: %v", err)
	}
	hooks := transformers.DefaultHookRegistry()
	if err := hooks.Process(schemaAfter); err != nil {
		t.Fatalf("hooks: %v", err)
	}

	return snapshotSchema(schemaBefore), snapshotSchema(schemaAfter)
}

func snapshotSchema(schema *ir.Schema) irSnapshot {
	out := irSnapshot{
		IRVersion: schema.IRVersion,
		Entities:  make([]string, 0),
		Services:  make([]serviceSnapshot, 0),
		Endpoints: make([]endpointSnapshot, 0),
		Repos:     make([]repoSnapshot, 0),
		Events:    make([]eventSnapshot, 0),
		Errors:    make([]errorSnapshot, 0),
	}

	for _, ent := range schema.Entities {
		out.Entities = append(out.Entities, ent.Name)
	}
	sort.Strings(out.Entities)

	for _, svc := range schema.Services {
		s := serviceSnapshot{
			Name:      svc.Name,
			Methods:   make([]string, 0),
			Uses:      append([]string{}, svc.Uses...),
			Publishes: append([]string{}, svc.Publishes...),
		}
		for _, m := range svc.Methods {
			s.Methods = append(s.Methods, m.Name)
		}
		sort.Strings(s.Methods)
		sort.Strings(s.Uses)
		sort.Strings(s.Publishes)
		out.Services = append(out.Services, s)
	}
	sort.Slice(out.Services, func(i, j int) bool { return out.Services[i].Name < out.Services[j].Name })

	for _, ep := range schema.Endpoints {
		e := endpointSnapshot{
			RPC:     ep.RPC,
			Method:  ep.Method,
			Path:    ep.Path,
			Service: ep.Service,
		}
		if ep.Auth != nil {
			e.AuthType = ep.Auth.Type
			if len(ep.Auth.Roles) > 0 {
				e.AuthRoles = append([]string{}, ep.Auth.Roles...)
				sort.Strings(e.AuthRoles)
			}
		}
		if ep.Pagination != nil {
			e.PaginationType = ep.Pagination.Type
		}
		out.Endpoints = append(out.Endpoints, e)
	}
	sort.Slice(out.Endpoints, func(i, j int) bool {
		if out.Endpoints[i].Service != out.Endpoints[j].Service {
			return out.Endpoints[i].Service < out.Endpoints[j].Service
		}
		if out.Endpoints[i].RPC != out.Endpoints[j].RPC {
			return out.Endpoints[i].RPC < out.Endpoints[j].RPC
		}
		return out.Endpoints[i].Method < out.Endpoints[j].Method
	})

	for _, repo := range schema.Repos {
		r := repoSnapshot{
			Name:    repo.Name,
			Entity:  repo.Entity,
			Finders: make([]string, 0),
		}
		for _, f := range repo.Finders {
			r.Finders = append(r.Finders, f.Name)
		}
		sort.Strings(r.Finders)
		out.Repos = append(out.Repos, r)
	}
	sort.Slice(out.Repos, func(i, j int) bool { return out.Repos[i].Name < out.Repos[j].Name })

	for _, ev := range schema.Events {
		e := eventSnapshot{Name: ev.Name, Fields: make([]string, 0)}
		for _, f := range ev.Fields {
			e.Fields = append(e.Fields, f.Name)
		}
		sort.Strings(e.Fields)
		out.Events = append(out.Events, e)
	}
	sort.Slice(out.Events, func(i, j int) bool { return out.Events[i].Name < out.Events[j].Name })

	for _, er := range schema.Errors {
		out.Errors = append(out.Errors, errorSnapshot{
			Name:       er.Name,
			Code:       er.Code,
			HTTPStatus: er.HTTPStatus,
		})
	}
	sort.Slice(out.Errors, func(i, j int) bool { return out.Errors[i].Name < out.Errors[j].Name })

	return out
}

func assertSnapshot(t *testing.T, path string, actual irSnapshot) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot %s: %v", path, err)
	}
	var expected irSnapshot
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("parse snapshot %s: %v", path, err)
	}
	if !equalJSON(expected, actual) {
		actualData, _ := json.MarshalIndent(actual, "", "  ")
		t.Fatalf("snapshot mismatch for %s\nactual:\n%s", path, string(actualData))
	}
}

func writeSnapshot(t *testing.T, path string, actual irSnapshot) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	data, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}

func equalJSON(a, b irSnapshot) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func mustLoadDomain(t *testing.T, p *parser.Parser, path string) cue.Value {
	t.Helper()
	val, err := p.LoadDomain(path)
	if err != nil {
		t.Fatalf("load domain %s: %v", path, err)
	}
	return val
}

func loadOptionalDomain(t *testing.T, p *parser.Parser, path string) (cue.Value, bool, error) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cue.Value{}, false, nil
	} else if err != nil {
		return cue.Value{}, false, err
	}
	val, err := p.LoadDomain(path)
	if err != nil {
		return cue.Value{}, false, err
	}
	return val, true, nil
}

func mustLoadOptionalDomain(t *testing.T, p *parser.Parser, path string) (cue.Value, bool) {
	t.Helper()
	val, ok, err := loadOptionalDomain(t, p, path)
	if err != nil {
		t.Fatalf("load optional domain %s: %v", path, err)
	}
	return val, ok
}
