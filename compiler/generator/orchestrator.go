package generator

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler"
)

// Step is an independent generator module unit.
// It declares required capabilities and a pure execution function.
type Step struct {
	Name        string
	ArtifactKey string
	Requires    []compiler.Capability
	Run         func() error
	// ParallelSafe allows adjacent independent steps to execute concurrently.
	// Steps touching shared emitter state or depending on previous artifacts must leave this false.
	ParallelSafe bool
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
	steps        []Step
	stepNames    map[string]struct{}
	artifactKeys map[string]string
	regErr       error
}

func NewStepRegistry() *StepRegistry {
	return &StepRegistry{
		steps:        make([]Step, 0, 64),
		stepNames:    make(map[string]struct{}, 64),
		artifactKeys: make(map[string]string, 64),
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
	artifactKey := strings.TrimSpace(step.ArtifactKey)
	if artifactKey != "" {
		if existingStep, exists := r.artifactKeys[artifactKey]; exists {
			if r.regErr == nil {
				r.regErr = fmt.Errorf("register step %q: duplicate artifact key %q already used by step %q", name, artifactKey, existingStep)
			}
			return
		}
		r.artifactKeys[artifactKey] = name
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
	for index := 0; index < len(steps); {
		if steps[index].ParallelSafe {
			end := index + 1
			for end < len(steps) && steps[end].ParallelSafe {
				end++
			}
			batch := steps[index:end]
			type result struct {
				events []StepEvent
				err    error
			}
			results := make([]result, len(batch))
			var wg sync.WaitGroup
			for i, step := range batch {
				wg.Add(1)
				go func(i int, step Step) {
					defer wg.Done()
					results[i].events, results[i].err = executeStep(td, caps, step, logger)
				}(i, step)
			}
			wg.Wait()
			for i, result := range results {
				for _, event := range result.events {
					if eventLogger != nil {
						eventLogger(event)
					}
				}
				if result.err != nil {
					return fmt.Errorf("target=%s step=%s: %w", td.Name, batch[i].Name, result.err)
				}
			}
			index = end
			continue
		}
		events, err := executeStep(td, caps, steps[index], logger)
		for _, event := range events {
			if eventLogger != nil {
				eventLogger(event)
			}
		}
		if err != nil {
			return fmt.Errorf("target=%s step=%s: %w", td.Name, steps[index].Name, err)
		}
		index++
	}
	return nil
}

func executeStep(td normalizer.TargetDef, caps compiler.CapabilitySet, step Step, logger func(string, ...interface{})) ([]StepEvent, error) {
	var events []StepEvent
	if !caps.HasAll(step.Requires...) {
		missing := caps.Missing(step.Requires...)
		if len(missing) > 0 && logger != nil {
			missingNames := make([]string, 0, len(missing))
			for _, c := range missing {
				missingNames = append(missingNames, string(c))
			}
			logger("Skipping %s for target %s: missing capabilities [%s]", step.Name, td.Name, strings.Join(missingNames, ", "))
		}
		events = append(events, StepEvent{
			Stage:       "emitters",
			Target:      td.Name,
			Step:        step.Name,
			Status:      "skip",
			MissingCaps: missing,
		})
		return events, nil
	}
	start := time.Now()
	events = append(events, StepEvent{
		Stage:  "emitters",
		Target: td.Name,
		Step:   step.Name,
		Status: "start",
	})
	if err := step.Run(); err != nil {
		events = append(events, StepEvent{
			Stage:      "emitters",
			Target:     td.Name,
			Step:       step.Name,
			Status:     "error",
			DurationMS: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		return events, err
	}
	events = append(events, StepEvent{
		Stage:      "emitters",
		Target:     td.Name,
		Step:       step.Name,
		Status:     "ok",
		DurationMS: time.Since(start).Milliseconds(),
	})
	return events, nil
}
