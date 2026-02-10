package emitter

import (
	"fmt"

	pyemitter "github.com/strogmv/ang/compiler/emitter/python"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/planner"
)

// EmitPythonFastAPIBackend generates a minimal FastAPI backend scaffold (M3 MVP).
func (e *Emitter) EmitPythonFastAPIBackend(
	entities []normalizer.Entity,
	services []normalizer.Service,
	endpoints []normalizer.Endpoint,
	repos []normalizer.Repository,
	project *normalizer.ProjectDef,
) error {
	var projectDef normalizer.ProjectDef
	if project != nil {
		projectDef = *project
	}
	schema := ir.ConvertFromNormalizer(
		entities, services, nil, nil, endpoints, repos,
		normalizer.ConfigDef{}, nil, nil, nil, nil, projectDef,
	)
	return e.EmitPythonFastAPIBackendFromIR(schema, e.Version)
}

func (e *Emitter) EmitPythonFastAPIBackendFromIR(schema *ir.Schema, fallbackVersion string) error {
	if err := ir.MigrateToCurrent(schema); err != nil {
		return fmt.Errorf("migrate ir schema: %w", err)
	}
	plan := planner.BuildFastAPIPlan(schema, fallbackVersion)
	return e.EmitPythonFastAPIBackendFromPlan(plan)
}

func (e *Emitter) EmitPythonFastAPIBackendFromPlan(plan planner.FastAPIPlan) error {
	rt := pyemitter.Runtime{
		Funcs:        e.getSharedFuncMap(),
		ReadTemplate: ReadTemplateByPath,
	}
	return pyemitter.EmitFastAPIBackend(e.OutputDir, rt, plan)
}
