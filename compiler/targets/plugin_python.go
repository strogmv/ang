package targets

import (
	"github.com/strogmv/ang/compiler"
	pyemitter "github.com/strogmv/ang/compiler/emitter/python"
	"github.com/strogmv/ang/compiler/generator"
)

type PythonPlugin struct{}

func (PythonPlugin) Name() string { return "python_fastapi" }

func (PythonPlugin) Capabilities() []compiler.Capability {
	return []compiler.Capability{
		compiler.CapabilityProfilePythonFastAPI,
	}
}

func (PythonPlugin) RegisterSteps(registry *generator.StepRegistry, ctx BuildContext) {
	pyemitter.Register(registry, pyemitter.RegisterInput{
		Em:          ctx.Emitter,
		IRSchema:    ctx.IRSchema,
		CfgDef:      ctx.Config,
		AuthDef:     ctx.Auth,
		RBACDef:     ctx.RBAC,
		InfraValues: ctx.InfraValues,
	})
}
