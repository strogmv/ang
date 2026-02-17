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

func TestBuildStepRegistry_ServiceImplStepPresenceMatchesGoPlugin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		project       *normalizer.ProjectDef
		expectedSteps int
	}{
		{
			name: "go plugin enabled via default set",
			project: &normalizer.ProjectDef{
				Plugins: []string{"shared", "go_legacy"},
			},
			expectedSteps: 1,
		},
		{
			name: "go plugin disabled",
			project: &normalizer.ProjectDef{
				Plugins: []string{"shared", "python_fastapi"},
			},
			expectedSteps: 0,
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
			if serviceImplSteps != tc.expectedSteps {
				t.Fatalf("expected %d \"Service Impls\" steps, got %d", tc.expectedSteps, serviceImplSteps)
			}
		})
	}
}

func TestBuildStepRegistry_CriticalArtifactKeysAreUnique(t *testing.T) {
	t.Parallel()

	reg, _, err := buildStepRegistry(buildStepRegistryInput{
		em:           &emitter.Emitter{},
		irSchema:     &ir.Schema{},
		projectDef:   &normalizer.ProjectDef{Plugins: []string{"shared", "go_legacy"}},
		targetOutput: OutputOptions{},
	})
	if err != nil {
		t.Fatalf("buildStepRegistry failed: %v", err)
	}

	seen := make(map[string]string)
	required := map[string]bool{
		"go:di_container":  false,
		"go:http_handlers": false,
		"go:frontend_sdk":  false,
		"go:service_impl":  false,
		"go:server_main":   false,
	}

	for _, step := range reg.Steps() {
		if step.ArtifactKey == "" {
			continue
		}
		if prev, exists := seen[step.ArtifactKey]; exists {
			t.Fatalf("duplicate artifact key %q: %q and %q", step.ArtifactKey, prev, step.Name)
		}
		seen[step.ArtifactKey] = step.Name
		if _, ok := required[step.ArtifactKey]; ok {
			required[step.ArtifactKey] = true
		}
	}

	for key, found := range required {
		if !found {
			t.Fatalf("expected artifact key %q to be present", key)
		}
	}
}
