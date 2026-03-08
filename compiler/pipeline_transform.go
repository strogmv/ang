package compiler

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
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

func validateFlowIntegrity(services []normalizer.Service) error {
	for _, svc := range services {
		for _, m := range svc.Methods {
			if len(m.Flow) == 0 {
				continue
			}
			declared := make(map[string]string)
			used := make(map[string]bool)
			for _, s := range m.Flow {
				for _, arg := range []string{"input", "value", "condition", "payload", "actor", "company"} {
					if val, ok := s.Args[arg].(string); ok {
						for name := range declared {
							if strings.Contains(val, name) {
								used[name] = true
							}
						}
					}
				}
				if out, ok := s.Args["output"].(string); ok && out != "" && out != "resp" {
					declared[out] = fmt.Sprintf("%s:%d", s.File, s.Line)
				}
			}
			for name, loc := range declared {
				if !used[name] {
					return fmt.Errorf("Logic Error: variable %s declared at %s is never used in method %s.%s", name, loc, svc.Name, m.Name)
				}
			}
		}
	}
	return nil
}
