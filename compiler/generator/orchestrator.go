package generator

import (
	"fmt"
	"strings"
	"time"

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

type StepEvent struct {
	Stage          string                `json:"stage"`
	Target         string                `json:"target"`
	Step           string                `json:"step"`
	Status         string                `json:"status"` // start|ok|skip|error
	DurationMS     int64                 `json:"duration_ms,omitempty"`
	MissingCaps    []compiler.Capability `json:"missing_caps,omitempty"`
	FilesGenerated int                   `json:"files_generated,omitempty"`
	Warnings       int                   `json:"warnings,omitempty"`
	Error          string                `json:"error,omitempty"`
}

type StepRegistry struct {
	steps     []Step
	stepNames map[string]struct{}
	regErr    error
}

func NewStepRegistry() *StepRegistry {
	return &StepRegistry{
		steps:     make([]Step, 0, 64),
		stepNames: make(map[string]struct{}, 64),
	}
}

func (r *StepRegistry) Register(step Step) {
	name := strings.TrimSpace(step.Name)
	if name == "" {
		if r.regErr == nil {
			r.regErr = fmt.Errorf("register step: empty name")
		}
		return
	}
	if _, exists := r.stepNames[name]; exists {
		if r.regErr == nil {
			r.regErr = fmt.Errorf("register step %q: duplicate step name (single active emitter path required)", name)
		}
		return
	}
	r.stepNames[name] = struct{}{}
	r.steps = append(r.steps, step)
}

func (r *StepRegistry) Steps() []Step {
	out := make([]Step, len(r.steps))
	copy(out, r.steps)
	return out
}

func (r *StepRegistry) Err() error {
	return r.regErr
}

func (r *StepRegistry) Execute(
	td normalizer.TargetDef,
	caps compiler.CapabilitySet,
	logger func(string, ...interface{}),
	eventLogger func(StepEvent),
) error {
	if r.regErr != nil {
		return r.regErr
	}
	return Execute(td, caps, r.steps, logger, eventLogger)
}

// Execute runs steps through capability gating.
// Missing capabilities skip the step with logger output.
func Execute(
	td normalizer.TargetDef,
	caps compiler.CapabilitySet,
	steps []Step,
	logger func(string, ...interface{}),
	eventLogger func(StepEvent),
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
			if eventLogger != nil {
				eventLogger(StepEvent{
					Stage:       "emitters",
					Target:      td.Name,
					Step:        step.Name,
					Status:      "skip",
					MissingCaps: missing,
				})
			}
			continue
		}
		start := time.Now()
		if eventLogger != nil {
			eventLogger(StepEvent{
				Stage:  "emitters",
				Target: td.Name,
				Step:   step.Name,
				Status: "start",
			})
		}
		if err := step.Run(); err != nil {
			if eventLogger != nil {
				eventLogger(StepEvent{
					Stage:      "emitters",
					Target:     td.Name,
					Step:       step.Name,
					Status:     "error",
					DurationMS: time.Since(start).Milliseconds(),
					Error:      err.Error(),
				})
			}
			return fmt.Errorf("target=%s step=%s: %w", td.Name, step.Name, err)
		}
		if eventLogger != nil {
			eventLogger(StepEvent{
				Stage:      "emitters",
				Target:     td.Name,
				Step:       step.Name,
				Status:     "ok",
				DurationMS: time.Since(start).Milliseconds(),
			})
		}
	}
	return nil
}
