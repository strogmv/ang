package goemitter

import (
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter/contracts"
	"github.com/strogmv/ang/compiler/generator"
)

type infraGoStepRunner func(RegisterInput) error

type infraGoStepDef struct {
	Runner infraGoStepRunner
}

var infraGoStepDefs = map[string]infraGoStepDef{}

func registerInfraGoStepRunner(key string, runner infraGoStepRunner) {
	if key == "" || runner == nil {
		return
	}
	infraGoStepDefs[key] = infraGoStepDef{
		Runner: runner,
	}
}

func runInfraGoStep(key string, in RegisterInput) error {
	def, ok := infraGoStepDefs[key]
	if !ok {
		return nil
	}
	return def.Runner(in)
}

func registerInfraGoSteps(registry *generator.StepRegistry, in RegisterInput) {
	steps := contracts.InfraStepsForValuesGo(in.InfraValues)
	for _, step := range steps {
		keyCopy := step.Key
		reqs := toCapabilities(step.Requires)
		registry.Register(generator.Step{
			Name:     step.Name,
			Requires: reqs,
			Run: func() error {
				return runInfraGoStep(keyCopy, in)
			},
		})
	}
}

func toCapabilities(requires []string) []compiler.Capability {
	out := make([]compiler.Capability, 0, len(requires))
	for _, r := range requires {
		out = append(out, compiler.Capability(r))
	}
	return out
}

func init() {
	registerInfraGoStepRunner(contracts.InfraKeyNotificationMuting, func(in RegisterInput) error {
		return in.Em.EmitNotificationMuting(contracts.InfraNotificationMuting(in.InfraValues), in.IRSchema)
	})
}
