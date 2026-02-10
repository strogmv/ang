package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func FuzzRunPipelineDeterministic(f *testing.F) {
	f.Add([]byte("alpha-seed"))
	f.Add([]byte("beta-seed-123"))
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	f.Fuzz(func(t *testing.T, seed []byte) {
		t.Helper()
		if len(seed) == 0 {
			seed = []byte{0}
		}

		tmp := t.TempDir()
		basePath := filepath.Join(tmp, "proj")
		if err := os.MkdirAll(filepath.Join(basePath, "cue", "domain"), 0o755); err != nil {
			t.Fatalf("mkdir domain: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(basePath, "cue", "api"), 0o755); err != nil {
			t.Fatalf("mkdir api: %v", err)
		}

		domainText, apiText := buildFuzzCueProject(seed)
		if err := os.WriteFile(filepath.Join(basePath, "cue", "domain", "fuzz_domain.cue"), []byte(domainText), 0o644); err != nil {
			t.Fatalf("write domain cue: %v", err)
		}
		if err := os.WriteFile(filepath.Join(basePath, "cue", "api", "fuzz_api.cue"), []byte(apiText), 0o644); err != nil {
			t.Fatalf("write api cue: %v", err)
		}

		first, firstWarnings := runPipelineCaptureWarnings(t, basePath)
		assertPipelineOutputValid(t, first)

		second, secondWarnings := runPipelineCaptureWarnings(t, basePath)
		assertPipelineOutputValid(t, second)

		if !reflect.DeepEqual(first, second) {
			t.Fatalf("RunPipeline output is not deterministic for seed=%q: %s", string(seed), explainPipelineDiff(first, second))
		}
		if !reflect.DeepEqual(firstWarnings, secondWarnings) {
			t.Fatalf("RunPipeline diagnostics are not deterministic for seed=%q", string(seed))
		}
	})
}

type pipelineSnapshot struct {
	Entities  []normalizer.Entity
	Services  []normalizer.Service
	Endpoints []normalizer.Endpoint
	Repos     []normalizer.Repository
	Events    []normalizer.EventDef
	Errors    []normalizer.ErrorDef
	Schedules []normalizer.ScheduleDef
	Scenarios []normalizer.ScenarioDef
}

func runPipelineCaptureWarnings(t *testing.T, basePath string) (pipelineSnapshot, []normalizer.Warning) {
	t.Helper()

	var warnings []normalizer.Warning
	entities, services, endpoints, repos, events, bizErrors, schedules, scenarios, err := RunPipelineWithOptions(basePath, PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			warnings = append(warnings, w)
		},
	})
	if err != nil {
		t.Fatalf("RunPipelineWithOptions failed: %v", err)
	}

	return pipelineSnapshot{
		Entities:  entities,
		Services:  services,
		Endpoints: endpoints,
		Repos:     repos,
		Events:    events,
		Errors:    bizErrors,
		Schedules: schedules,
		Scenarios: scenarios,
	}, warnings
}

func assertPipelineOutputValid(t *testing.T, out pipelineSnapshot) {
	t.Helper()

	serviceNames := make(map[string]struct{}, len(out.Services))
	for _, s := range out.Services {
		if strings.TrimSpace(s.Name) == "" {
			t.Fatalf("invalid output: empty service name")
		}
		serviceNames[s.Name] = struct{}{}
	}

	entityNames := make(map[string]struct{}, len(out.Entities))
	for _, e := range out.Entities {
		if strings.TrimSpace(e.Name) == "" {
			t.Fatalf("invalid output: empty entity name")
		}
		entityNames[e.Name] = struct{}{}
	}

	for _, ep := range out.Endpoints {
		if strings.TrimSpace(ep.Method) == "" {
			t.Fatalf("invalid output: endpoint has empty method")
		}
		if strings.TrimSpace(ep.Path) == "" {
			t.Fatalf("invalid output: endpoint has empty path")
		}
		if strings.TrimSpace(ep.ServiceName) == "" {
			t.Fatalf("invalid output: endpoint has empty service")
		}
		if _, ok := serviceNames[ep.ServiceName]; !ok {
			t.Fatalf("invalid output: endpoint references unknown service %q", ep.ServiceName)
		}
		if strings.TrimSpace(ep.RPC) == "" {
			t.Fatalf("invalid output: endpoint has empty RPC")
		}
	}

	for _, r := range out.Repos {
		if strings.TrimSpace(r.Entity) == "" {
			t.Fatalf("invalid output: repository has empty entity")
		}
		if _, ok := entityNames[r.Entity]; !ok {
			t.Fatalf("invalid output: repository references unknown entity %q", r.Entity)
		}
	}
}

type fuzzGen struct {
	state uint64
}

func newFuzzGen(seed []byte) *fuzzGen {
	var s uint64 = 0x9e3779b97f4a7c15
	for _, b := range seed {
		s ^= uint64(b) + 0x9e3779b97f4a7c15 + (s << 6) + (s >> 2)
	}
	if s == 0 {
		s = 1
	}
	return &fuzzGen{state: s}
}

func (g *fuzzGen) next() uint64 {
	x := g.state
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	g.state = x
	return x
}

func (g *fuzzGen) intn(n int) int {
	if n <= 1 {
		return 0
	}
	return int(g.next() % uint64(n))
}

func buildFuzzCueProject(seed []byte) (domainCue string, apiCue string) {
	g := newFuzzGen(seed)

	entityCount := 1 + g.intn(4)  // 1..4
	serviceCount := 1 + g.intn(3) // 1..3
	opCount := 1 + g.intn(8)      // 1..8

	type entitySpec struct {
		name   string
		fields []fieldSpec
	}
	type opSpec struct {
		name    string
		service string
		method  string
		path    string
	}

	entities := make([]entitySpec, 0, entityCount)
	for i := 0; i < entityCount; i++ {
		name := fmt.Sprintf("#Ent%d", i+1)
		fieldN := 1 + g.intn(4) // 1..4 extra fields + id
		fields := []fieldSpec{{name: "id", cueType: "string"}}
		for j := 0; j < fieldN; j++ {
			fields = append(fields, fieldSpec{
				name:    fmt.Sprintf("f%d_%d", i+1, j+1),
				cueType: randomCueType(g),
			})
		}
		entities = append(entities, entitySpec{name: name, fields: fields})
	}

	services := make([]string, 0, serviceCount)
	for i := 0; i < serviceCount; i++ {
		services = append(services, fmt.Sprintf("svc%d", i+1))
	}

	ops := make([]opSpec, 0, opCount)
	for i := 0; i < opCount; i++ {
		name := fmt.Sprintf("Op%d", i+1)
		svc := services[g.intn(len(services))]
		methods := []string{"GET", "POST", "PATCH", "DELETE"}
		m := methods[g.intn(len(methods))]
		path := fmt.Sprintf("/api/%s/%d", strings.ToLower(svc), i+1)
		if m == "GET" && g.intn(2) == 1 {
			path += "/{id}"
		}
		ops = append(ops, opSpec{name: name, service: svc, method: m, path: path})
	}

	var d strings.Builder
	d.WriteString("package domain\n\n")
	for _, e := range entities {
		d.WriteString(e.name)
		d.WriteString(": {\n")
		for _, f := range e.fields {
			d.WriteString("    ")
			d.WriteString(f.name)
			d.WriteString(": ")
			d.WriteString(f.cueType)
			d.WriteString("\n")
		}
		d.WriteString("}\n\n")
	}

	var a strings.Builder
	a.WriteString("package api\n\n")
	for _, op := range ops {
		a.WriteString(op.name)
		a.WriteString(": {\n")
		a.WriteString("    service: \"")
		a.WriteString(op.service)
		a.WriteString("\"\n")
		a.WriteString("    input: {\n")
		a.WriteString("        id: string\n")
		if g.intn(2) == 1 {
			a.WriteString("        count?: int\n")
		}
		a.WriteString("    }\n")
		a.WriteString("    output: {\n")
		a.WriteString("        ok: bool\n")
		a.WriteString("    }\n")
		a.WriteString("}\n\n")
	}

	a.WriteString("HTTP: {\n")
	for _, op := range ops {
		a.WriteString("    ")
		a.WriteString(op.name)
		a.WriteString(": {\n")
		a.WriteString("        method: \"")
		a.WriteString(op.method)
		a.WriteString("\"\n")
		a.WriteString("        path: \"")
		a.WriteString(op.path)
		a.WriteString("\"\n")
		a.WriteString("    }\n")
	}
	a.WriteString("}\n")

	return d.String(), a.String()
}

type fieldSpec struct {
	name    string
	cueType string
}

func randomCueType(g *fuzzGen) string {
	types := []string{"string", "int", "bool"}
	return types[g.intn(len(types))]
}

func explainPipelineDiff(first, second pipelineSnapshot) string {
	if !reflect.DeepEqual(first.Entities, second.Entities) {
		return fmt.Sprintf("entities differ: %d vs %d", len(first.Entities), len(second.Entities))
	}
	if !reflect.DeepEqual(first.Services, second.Services) {
		return fmt.Sprintf("services differ: %d vs %d", len(first.Services), len(second.Services))
	}
	if !reflect.DeepEqual(first.Endpoints, second.Endpoints) {
		return fmt.Sprintf("endpoints differ: %d vs %d", len(first.Endpoints), len(second.Endpoints))
	}
	if !reflect.DeepEqual(first.Repos, second.Repos) {
		return fmt.Sprintf("repos differ: %d vs %d", len(first.Repos), len(second.Repos))
	}
	if !reflect.DeepEqual(first.Events, second.Events) {
		return fmt.Sprintf("events differ: %d vs %d", len(first.Events), len(second.Events))
	}
	if !reflect.DeepEqual(first.Errors, second.Errors) {
		return fmt.Sprintf("errors differ: %d vs %d", len(first.Errors), len(second.Errors))
	}
	if !reflect.DeepEqual(first.Schedules, second.Schedules) {
		return fmt.Sprintf("schedules differ: %d vs %d", len(first.Schedules), len(second.Schedules))
	}
	if !reflect.DeepEqual(first.Scenarios, second.Scenarios) {
		return fmt.Sprintf("scenarios differ: %d vs %d", len(first.Scenarios), len(second.Scenarios))
	}
	return "unknown snapshot mismatch"
}
