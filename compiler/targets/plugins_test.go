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

func TestResolvePlugins_DefaultsToBuiltins(t *testing.T) {
	t.Parallel()

	plugins, err := ResolvePlugins(nil)
	if err != nil {
		t.Fatalf("resolve default plugins: %v", err)
	}
	if len(plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(plugins))
	}
}

func TestResolvePlugins_FromProjectList(t *testing.T) {
	t.Parallel()

	plugins, err := ResolvePlugins(&normalizer.ProjectDef{
		Plugins: []string{"shared", "go_legacy"},
	})
	if err != nil {
		t.Fatalf("resolve project plugins: %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
	if plugins[0].Name() != "shared" || plugins[1].Name() != "go_legacy" {
		t.Fatalf("unexpected plugin order: %s, %s", plugins[0].Name(), plugins[1].Name())
	}
}

func TestResolvePlugins_UnknownPlugin(t *testing.T) {
	t.Parallel()

	_, err := ResolvePlugins(&normalizer.ProjectDef{
		Plugins: []string{"shared", "unknown_plugin"},
	})
	if err == nil {
		t.Fatalf("expected error for unknown plugin")
	}
}
