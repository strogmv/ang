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

func (p PythonPlugin) Descriptor() PluginDescriptor {
	return PluginDescriptor{
		SDKVersion:   PluginSDKV2,
		Capabilities: p.Capabilities(),
		Compatibility: PluginCompatibility{
			MinANGVersion:           "0.1.0",
			SupportedSchemaVersions: []string{compiler.SchemaVersion},
		},
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
