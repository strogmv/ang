package targets

import (
	"github.com/strogmv/ang/compiler"
	sharedsteps "github.com/strogmv/ang/compiler/emitter/shared"
	"github.com/strogmv/ang/compiler/generator"
)

type SharedPlugin struct{}

func (SharedPlugin) Name() string { return "shared" }

func (SharedPlugin) Capabilities() []compiler.Capability {
	return []compiler.Capability{
		compiler.CapabilityHTTP,
		compiler.CapabilityEvents,
	}
}

func (SharedPlugin) RegisterSteps(registry *generator.StepRegistry, ctx BuildContext) {
	sharedsteps.Register(registry, sharedsteps.RegisterInput{
		Em:               ctx.Emitter,
		IRSchema:         ctx.IRSchema,
		ProjectDef:       ctx.Project,
		PythonSDKEnabled: ctx.PythonSDKEnabled,
	})
}
