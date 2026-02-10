package generator

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
)

// Step is an independent generator module unit.
// It declares required capabilities and a pure execution function.
type Step struct {
	Name     string
	Requires []compiler.Capability
	Run      func() error
}

type StepRegistry struct {
	steps []Step
}

func NewStepRegistry() *StepRegistry {
	return &StepRegistry{steps: make([]Step, 0, 64)}
}

func (r *StepRegistry) Register(step Step) {
	r.steps = append(r.steps, step)
}

func (r *StepRegistry) Steps() []Step {
	out := make([]Step, len(r.steps))
	copy(out, r.steps)
	return out
}

func (r *StepRegistry) Execute(
	td normalizer.TargetDef,
	caps compiler.CapabilitySet,
	logger func(string, ...interface{}),
) error {
	return Execute(td, caps, r.steps, logger)
}

// Execute runs steps through capability gating.
// Missing capabilities skip the step with logger output.
func Execute(
	td normalizer.TargetDef,
	caps compiler.CapabilitySet,
	steps []Step,
	logger func(string, ...interface{}),
) error {
	for _, step := range steps {
		if !caps.HasAll(step.Requires...) {
			missing := caps.Missing(step.Requires...)
			if len(missing) > 0 && logger != nil {
				missingNames := make([]string, 0, len(missing))
				for _, c := range missing {
					missingNames = append(missingNames, string(c))
				}
				logger("Skipping %s for target %s: missing capabilities [%s]", step.Name, td.Name, strings.Join(missingNames, ", "))
			}
			continue
		}
		if err := step.Run(); err != nil {
			return fmt.Errorf("target=%s step=%s: %w", td.Name, step.Name, err)
		}
	}
	return nil
}
