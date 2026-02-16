package main

import (
	"testing"

	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestBuildStepRegistry_HasSingleServiceImplEmitterStep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		project *normalizer.ProjectDef
	}{
		{
			name:    "default plugins",
			project: nil,
		},
		{
			name: "duplicate go plugin entries are deduplicated",
			project: &normalizer.ProjectDef{
				Plugins: []string{"go_legacy", "go_legacy", "shared"},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reg, _, err := buildStepRegistry(buildStepRegistryInput{
				em:           &emitter.Emitter{},
				irSchema:     &ir.Schema{},
				projectDef:   tc.project,
				targetOutput: OutputOptions{},
			})
			if err != nil {
				t.Fatalf("buildStepRegistry failed: %v", err)
			}

			serviceImplSteps := 0
			for _, step := range reg.Steps() {
				if step.Name == "Service Impls" {
					serviceImplSteps++
				}
			}
			if serviceImplSteps != 1 {
				t.Fatalf("expected exactly one \"Service Impls\" step, got %d", serviceImplSteps)
			}
		})
	}
}

