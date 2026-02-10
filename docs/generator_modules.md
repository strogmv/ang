# Generator Modules

ANG build orchestration uses independent generator modules (steps) with capability gates.

Core contract:

- `compiler/generator/orchestrator.go`
- `generator.Step { Name, Requires, Run }`
- `generator.Execute(...)`

This separates:

- pipeline orchestration (step execution, capability checks)
- emitters (implementation of generation logic)

## Design Rule

Emitter methods must not decide *whether* they run by language branches in templates.
Selection belongs to orchestrator + capability matrix.

## Current Integration

- `cmd/ang/main.go` builds module lists for target profiles.
- Execution is delegated to `generator.Execute`.
- Missing capabilities skip steps with explicit logs.

