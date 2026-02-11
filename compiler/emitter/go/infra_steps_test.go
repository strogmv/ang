package goemitter

import (
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestInfraGoStepRunnerRegistryContainsNotificationMuting(t *testing.T) {
	t.Parallel()

	def, ok := infraGoStepDefs[normalizer.InfraKeyNotificationMuting]
	if !ok {
		t.Fatalf("expected runner for key %q", normalizer.InfraKeyNotificationMuting)
	}
	if def.Runner == nil {
		t.Fatalf("expected non-nil runner for key %q", normalizer.InfraKeyNotificationMuting)
	}
	steps := normalizer.NewInfraRegistry().StepsForValues(normalizer.InfraLanguageGo, map[string]any{
		normalizer.InfraKeyNotificationMuting: &normalizer.NotificationMutingDef{Enabled: true},
	})
	if len(steps) != 1 {
		t.Fatalf("expected one infra step from registry, got %d", len(steps))
	}
	if steps[0].Name != "Notification Muting" {
		t.Fatalf("unexpected step name %q", steps[0].Name)
	}
	caps := toCapabilities(steps[0].Requires)
	if len(caps) != 1 || caps[0] != compiler.CapabilityProfileGoLegacy {
		t.Fatalf("unexpected capabilities: %+v", caps)
	}
}

func TestRunInfraGoStepUnknownKeyIsNoop(t *testing.T) {
	t.Parallel()

	if err := runInfraGoStep("unknown", RegisterInput{}); err != nil {
		t.Fatalf("expected noop for unknown key, got error: %v", err)
	}
}

func TestRegisterInfraGoStepsUsesLocalManifest(t *testing.T) {
	t.Parallel()

	reg := generator.NewStepRegistry()
	registerInfraGoSteps(reg, RegisterInput{
		InfraValues: map[string]any{
			normalizer.InfraKeyNotificationMuting: &normalizer.NotificationMutingDef{Enabled: true},
		},
	})
	steps := reg.Steps()
	if len(steps) != 1 {
		t.Fatalf("expected 1 registered infra step, got %d", len(steps))
	}
}
