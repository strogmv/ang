package compiler

import (
	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/transformers"
)

func ConvertAndTransform(
	entities []normalizer.Entity, services []normalizer.Service, events []normalizer.EventDef,
	errors []normalizer.ErrorDef, endpoints []normalizer.Endpoint, scopes []normalizer.ScopeDef, repos []normalizer.Repository,
	config normalizer.ConfigDef, auth *normalizer.AuthDef, rbac *normalizer.RBACDef,
	schedules []normalizer.ScheduleDef, views []normalizer.ViewDef, project normalizer.ProjectDef,
) (*ir.Schema, error) {
	schema := ir.ConvertFromNormalizer(entities, services, events, errors, endpoints, scopes, repos, config, auth, rbac, schedules, views, project)
	if err := ir.MigrateToCurrent(schema); err != nil {
		return nil, WrapContractError(StageIR, ErrCodeIRVersionMigration, "migrate ir schema", err)
	}

	registry := transformers.DefaultRegistry()
	if err := registry.Apply(schema); err != nil {
		return nil, WrapContractError(StageTransformers, ErrCodeTransformerApply, "apply transformers", err)
	}

	hooks := transformers.DefaultHookRegistry()
	if err := hooks.Process(schema); err != nil {
		return nil, WrapContractError(StageTransformers, ErrCodeHookProcess, "process hooks", err)
	}
	if err := ir.ValidateABIV2(schema); err != nil {
		return nil, WrapContractError(StageIR, ErrCodeIRVersionMigration, "validate ir abi v2", err)
	}

	return schema, nil
}
