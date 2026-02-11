package targets

import (
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

// BuiltinPlugins returns default in-process plugins in deterministic order.
func BuiltinPlugins() []TargetPlugin {
	return []TargetPlugin{
		SharedPlugin{},
		PythonPlugin{},
		GoPlugin{},
	}
}
