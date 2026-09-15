package goemitter

import (
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/generator"
)

// Steps whose output a project may not use require a usage capability and
// remove their earlier output when skipped.
func TestRegisterGatesUnusedOutputAndCleansItUp(t *testing.T) {
	registry := generator.NewStepRegistry()
	Register(registry, RegisterInput{})
	byName := map[string]generator.Step{}
	for _, step := range registry.Steps() {
		byName[step.Name] = step
	}
	for _, name := range []string{
		"DI Container", "EU AI Act Compliance", "NIS2 Compliance", "DTOs", "Repo Stubs",
		"SQLC Config", "SQL Queries", "S3 Client", "Mongo Repos", "Mongo Common", "Mongo Schemas",
		"Redis Client", "Refresh Store Memory", "Refresh Store Postgres", "Refresh Store Hybrid",
	} {
		step, ok := byName[name]
		if !ok {
			t.Errorf("step %q is not registered", name)
			continue
		}
		gated := false
		for _, capability := range step.Requires {
			if compiler.IsUsageCapability(capability) {
				gated = true
			}
		}
		if !gated {
			t.Errorf("%s requires no usage capability: %v", name, step.Requires)
		}
		if step.OnSkip == nil {
			t.Errorf("%s has no OnSkip cleanup", name)
		}
	}
	// http_common imports these unconditionally.
	for _, name := range []string{"Redis StateStore", "Refresh Store Redis"} {
		for _, capability := range byName[name].Requires {
			if compiler.IsUsageCapability(capability) {
				t.Errorf("%s must stay ungated, requires %v", name, byName[name].Requires)
			}
		}
	}
}
