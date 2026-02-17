package targets

import (
	"fmt"
	"sort"
	"strconv"
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

// PluginSDKV2 is the stable plugin contract generation used by ANG.
const PluginSDKV2 = "v2"

// PluginCompatibility defines which ANG core versions and schema versions are supported.
type PluginCompatibility struct {
	MinANGVersion           string
	MaxANGVersion           string
	SupportedSchemaVersions []string
}

// PluginDescriptor is a declared plugin contract for capability/compatibility checks.
type PluginDescriptor struct {
	SDKVersion    string
	Capabilities  []compiler.Capability
	Compatibility PluginCompatibility
}

// TargetPluginV2 is the recommended stable plugin contract.
type TargetPluginV2 interface {
	TargetPlugin
	Descriptor() PluginDescriptor
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

func descriptorForPlugin(plugin TargetPlugin) (PluginDescriptor, error) {
	p2, ok := plugin.(TargetPluginV2)
	if !ok {
		return PluginDescriptor{}, fmt.Errorf("plugin %q uses legacy SDK; implement Descriptor() for %s", plugin.Name(), PluginSDKV2)
	}
	desc := p2.Descriptor()
	if strings.TrimSpace(desc.SDKVersion) == "" {
		desc.SDKVersion = PluginSDKV2
	}
	if len(desc.Capabilities) == 0 {
		desc.Capabilities = append([]compiler.Capability(nil), plugin.Capabilities()...)
	}
	return desc, nil
}

func validateDescriptor(pluginName string, desc PluginDescriptor) error {
	if strings.TrimSpace(desc.SDKVersion) != PluginSDKV2 {
		return fmt.Errorf("plugin %q declares unsupported SDK version %q (expected %q)", pluginName, desc.SDKVersion, PluginSDKV2)
	}
	seen := map[compiler.Capability]bool{}
	for _, cap := range desc.Capabilities {
		if strings.TrimSpace(string(cap)) == "" {
			return fmt.Errorf("plugin %q declares empty capability", pluginName)
		}
		if seen[cap] {
			return fmt.Errorf("plugin %q declares duplicated capability %q", pluginName, cap)
		}
		seen[cap] = true
	}
	return nil
}

func parseVersion(raw string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) < 2 || len(parts) > 3 {
		return out, fmt.Errorf("invalid version %q", raw)
	}
	for i := 0; i < len(parts); i++ {
		v, err := strconv.Atoi(parts[i])
		if err != nil || v < 0 {
			return out, fmt.Errorf("invalid version %q", raw)
		}
		out[i] = v
	}
	return out, nil
}

func compareVersion(a, b string) (int, error) {
	av, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	bv, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if av[i] < bv[i] {
			return -1, nil
		}
		if av[i] > bv[i] {
			return 1, nil
		}
	}
	return 0, nil
}

func isVersionAllowed(current, min, max string) (bool, error) {
	if strings.TrimSpace(min) != "" {
		cmp, err := compareVersion(current, min)
		if err != nil {
			return false, err
		}
		if cmp < 0 {
			return false, nil
		}
	}
	if strings.TrimSpace(max) != "" {
		cmp, err := compareVersion(current, max)
		if err != nil {
			return false, err
		}
		if cmp > 0 {
			return false, nil
		}
	}
	return true, nil
}

func isSchemaAllowed(current string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		if strings.TrimSpace(item) == current {
			return true
		}
	}
	return false
}

func validateCompatibility(pluginName string, desc PluginDescriptor, angVersion, schemaVersion string) error {
	ok, err := isVersionAllowed(angVersion, desc.Compatibility.MinANGVersion, desc.Compatibility.MaxANGVersion)
	if err != nil {
		return fmt.Errorf("plugin %q compatibility check failed: %w", pluginName, err)
	}
	if !ok {
		return fmt.Errorf(
			"plugin %q incompatible with ANG %s (supported min=%q max=%q)",
			pluginName, angVersion, desc.Compatibility.MinANGVersion, desc.Compatibility.MaxANGVersion,
		)
	}
	if !isSchemaAllowed(schemaVersion, desc.Compatibility.SupportedSchemaVersions) {
		return fmt.Errorf(
			"plugin %q incompatible with schema %s (supported: %s)",
			pluginName, schemaVersion, strings.Join(desc.Compatibility.SupportedSchemaVersions, ", "),
		)
	}
	return nil
}

func validatePluginContract(plugin TargetPlugin) error {
	desc, err := descriptorForPlugin(plugin)
	if err != nil {
		return err
	}
	if err := validateDescriptor(plugin.Name(), desc); err != nil {
		return err
	}
	if err := validateCompatibility(plugin.Name(), desc, compiler.Version, compiler.SchemaVersion); err != nil {
		return err
	}
	return nil
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
			plugin := pluginRegistry[name]
			if err := validatePluginContract(plugin); err != nil {
				return nil, err
			}
			out = append(out, plugin)
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
		if err := validatePluginContract(plugin); err != nil {
			return nil, err
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
