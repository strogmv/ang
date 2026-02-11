package targets

import (
	"testing"

	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestBuiltinPlugins_OrderAndNames(t *testing.T) {
	t.Parallel()

	plugins := BuiltinPlugins()
	if len(plugins) != 3 {
		t.Fatalf("expected 3 builtin plugins, got %d", len(plugins))
	}
	if plugins[0].Name() != "shared" {
		t.Fatalf("expected first plugin shared, got %q", plugins[0].Name())
	}
	if plugins[1].Name() != "python_fastapi" {
		t.Fatalf("expected second plugin python_fastapi, got %q", plugins[1].Name())
	}
	if plugins[2].Name() != "go_legacy" {
		t.Fatalf("expected third plugin go_legacy, got %q", plugins[2].Name())
	}
}

func TestBuiltinPlugins_RegisterStepsSmoke(t *testing.T) {
	t.Parallel()

	reg := generator.NewStepRegistry()
	ctx := BuildContext{
		InfraValues: map[string]any{
			normalizer.InfraKeyAuth:               &normalizer.AuthDef{},
			normalizer.InfraKeyNotificationMuting: &normalizer.NotificationMutingDef{Enabled: true},
		},
	}
	for _, plugin := range BuiltinPlugins() {
		plugin.RegisterSteps(reg, ctx)
	}
	if len(reg.Steps()) == 0 {
		t.Fatalf("expected registered steps from builtin plugins")
	}
}
