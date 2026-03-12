package emitter

import (
	"testing"

	"github.com/strogmv/ang-ir/ir"
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
