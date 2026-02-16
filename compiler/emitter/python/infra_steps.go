package python

import (
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter/contracts"
	"github.com/strogmv/ang/compiler/generator"
)

type infraPythonStepRunner func(RegisterInput) error

type infraPythonStepDef struct {
	Runner infraPythonStepRunner
}

var infraPythonStepDefs = map[string]infraPythonStepDef{}

func registerInfraPythonStepRunner(key string, runner infraPythonStepRunner) {
	if key == "" || runner == nil {
		return
	}
	infraPythonStepDefs[key] = infraPythonStepDef{
		Runner: runner,
	}
}

func registerInfraPythonSteps(registry *generator.StepRegistry, in RegisterInput) {
	steps := contracts.InfraStepsForValuesPython(in.InfraValues)
	for _, step := range steps {
		keyCopy := step.Key
		registry.Register(generator.Step{
			Name:     step.Name,
			Requires: toCapabilities(step.Requires),
			Run: func() error {
				return runInfraPythonStep(keyCopy, in)
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

func runInfraPythonStep(key string, in RegisterInput) error {
	def, ok := infraPythonStepDefs[key]
	if !ok || def.Runner == nil {
		return nil
	}
	return def.Runner(in)
}

func init() {
	registerInfraPythonStepRunner(contracts.InfraKeyAuth, func(in RegisterInput) error {
		return in.Em.EmitPythonAuthStores(in.AuthDef)
	})
}
