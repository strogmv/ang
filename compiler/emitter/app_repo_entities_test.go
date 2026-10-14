package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

func TestHasRepoEntitiesIR_DetectsFlowRepoSteps(t *testing.T) {
	e := &Emitter{}
	funcs := e.getAppFuncMap()

	hasRepo, ok := funcs["HasRepoEntitiesIR"].(func([]ir.Service, []ir.Entity) bool)
	if !ok {
		t.Fatal("HasRepoEntitiesIR func missing or wrong signature")
	}

	services := []ir.Service{{
		Name: "Sandbox",
		Methods: []ir.Method{{
			Name: "ListProjects",
			Flow: []ir.FlowStep{{
				Action: "repo.ListAll",
				Args: map[string]interface{}{
					"source": "Project",
				},
			}},
		}},
	}}
	entities := []ir.Entity{{
		Name:     "Project",
		Metadata: map[string]interface{}{},
	}}

	if !hasRepo(services, entities) {
		t.Fatal("expected HasRepoEntitiesIR=true for repo.* flow usage")
	}
}

// The runtime container passes repositories to New<Service>Impl by position,
// so its list must be the constructor's list: sources, repo.* steps,
// audit.Log, and repositories the Go code reaches (s.TicketRepo).
func TestRepoEntityListsOfContainerMatchTheConstructor(t *testing.T) {
	e := &Emitter{}
	funcs := e.getAppFuncMap()
	perService := funcs["getRepoEntitiesIR"].(func(ir.Service, []ir.Entity) []string)
	allUsed := funcs["AllUsedRepoEntitiesIR"].(func([]ir.Service, []ir.Entity) []string)
	hasRepo := funcs["HasRepoEntitiesIR"].(func([]ir.Service, []ir.Entity) bool)

	svc := ir.Service{
		Name: "Stats",
		Methods: []ir.Method{
			{Name: "CountRuns", Impl: &ir.Impl{Lang: "go", Code: "runs, _ := s.RunCandidateRepo.ListAll(ctx)\n_ = runs"}},
			{Name: "Close", Flow: []ir.FlowStep{{Action: "logic.Call", Args: map[string]any{"func": "s.TicketRepo.Delete", "args": []any{"ctx", "req.ID"}}}}},
			{Name: "Audit", Flow: []ir.FlowStep{{Action: "audit.Log", Args: map[string]any{}}}},
			{Name: "Get", Sources: []ir.Source{{Name: "project", Entity: "Project"}}},
		},
	}
	entities := []ir.Entity{
		{Name: "RunCandidate", Metadata: map[string]any{}},
		{Name: "Ticket", Metadata: map[string]any{}},
		{Name: "AuditLog", Metadata: map[string]any{}},
		{Name: "Project", Metadata: map[string]any{}},
		{Name: "Unused", Metadata: map[string]any{}},
	}
	want := []string{"AuditLog", "Project", "RunCandidate", "Ticket"}
	ctor := serviceImplRepoEntities(IRServiceToNormalizer(svc), IREntitiesToNormalizer(entities))
	if strings.Join(ctor, ",") != strings.Join(want, ",") {
		t.Fatalf("constructor repositories = %v, want %v", ctor, want)
	}
	if got := perService(svc, entities); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("getRepoEntitiesIR = %v, want %v", got, want)
	}
	if got := allUsed([]ir.Service{svc}, entities); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("AllUsedRepoEntitiesIR = %v, want %v", got, want)
	}
	codeOnly := ir.Service{Name: "Stats", Methods: svc.Methods[:1]}
	if !hasRepo([]ir.Service{codeOnly}, entities) {
		t.Error("HasRepoEntitiesIR misses a repository only Go code reaches")
	}
}
