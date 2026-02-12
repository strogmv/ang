package targets

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// BuildContext carries all data required by target plugins to register steps.
type BuildContext struct {
	Emitter             *emitter.Emitter
	IRSchema            *ir.Schema
	MainContext         emitter.MainContext
	Scenarios           []normalizer.ScenarioDef
	Config              *normalizer.ConfigDef
	Auth                *normalizer.AuthDef
	RBAC                *normalizer.RBACDef
	InfraValues         map[string]any
	EmailTemplates      []normalizer.EmailTemplateDef
	Project             *normalizer.ProjectDef
	PythonSDKEnabled    bool
	IsMicroservice      bool
	TestStubsEnabled    bool
	ResolveMissingTests func() ([]normalizer.Endpoint, error)
	CopyFrontendSDK     func() error
	CopyFrontendAdmin   func() error
	WriteFrontendEnv    func() error
}

// TargetPlugin is an extension point for language/platform emitters.
type TargetPlugin interface {
	Name() string
	Capabilities() []compiler.Capability
	RegisterSteps(registry *generator.StepRegistry, ctx BuildContext)
}

var (
	pluginOrder    []string
	pluginRegistry = map[string]TargetPlugin{}
)

func registerBuiltinPlugins() {
	for _, plugin := range BuiltinPlugins() {
		_ = registerPluginInternal(plugin)
	}
}

func registerPluginInternal(plugin TargetPlugin) error {
	if plugin == nil {
		return fmt.Errorf("nil plugin")
	}
	name := strings.TrimSpace(plugin.Name())
	if name == "" {
		return fmt.Errorf("plugin with empty name")
	}
	if _, exists := pluginRegistry[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}
	pluginRegistry[name] = plugin
	pluginOrder = append(pluginOrder, name)
	return nil
}

// RegisterPlugin registers an in-process plugin.
func RegisterPlugin(plugin TargetPlugin) error {
	return registerPluginInternal(plugin)
}

// ResolvePlugins resolves active plugins from project config.
// If project or project.Plugins is empty, all registered plugins are returned in registration order.
func ResolvePlugins(project *normalizer.ProjectDef) ([]TargetPlugin, error) {
	if len(pluginOrder) == 0 {
		registerBuiltinPlugins()
	}
	if project == nil || len(project.Plugins) == 0 {
		out := make([]TargetPlugin, 0, len(pluginOrder))
		for _, name := range pluginOrder {
			out = append(out, pluginRegistry[name])
		}
		return out, nil
	}

	out := make([]TargetPlugin, 0, len(project.Plugins))
	seen := make(map[string]bool, len(project.Plugins))
	for _, name := range project.Plugins {
		key := strings.TrimSpace(name)
		if key == "" || seen[key] {
			continue
		}
		plugin, ok := pluginRegistry[key]
		if !ok {
			available := append([]string(nil), pluginOrder...)
			sort.Strings(available)
			return nil, fmt.Errorf("unknown plugin %q (available: %s)", key, strings.Join(available, ", "))
		}
		seen[key] = true
		out = append(out, plugin)
	}
	return out, nil
}

func init() {
	registerBuiltinPlugins()
}

// BuiltinPlugins returns default in-process plugins in deterministic order.
func BuiltinPlugins() []TargetPlugin {
	return []TargetPlugin{
		SharedPlugin{},
		PythonPlugin{},
		GoPlugin{},
	}
}
