package targets

import (
	"github.com/strogmv/ang/compiler"
	goemitter "github.com/strogmv/ang/compiler/emitter/go"
	"github.com/strogmv/ang/compiler/generator"
)

type GoPlugin struct{}

func (GoPlugin) Name() string { return "go_legacy" }

func (GoPlugin) Capabilities() []compiler.Capability {
	return []compiler.Capability{
		compiler.CapabilityProfileGoLegacy,
	}
}

func (GoPlugin) RegisterSteps(registry *generator.StepRegistry, ctx BuildContext) {
	goemitter.Register(registry, goemitter.RegisterInput{
		Em:                      ctx.Emitter,
		IRSchema:                ctx.IRSchema,
		Ctx:                     ctx.MainContext,
		Scenarios:               ctx.Scenarios,
		CfgDef:                  ctx.Config,
		AuthDef:                 ctx.Auth,
		RBACDef:                 ctx.RBAC,
		InfraValues:             ctx.InfraValues,
		IsMicroservice:          ctx.IsMicroservice,
		TestStubsEnabled:        ctx.TestStubsEnabled,
		ResolveMissingTestStubs: ctx.ResolveMissingTests,
		CopyFrontendSDK:         ctx.CopyFrontendSDK,
		CopyFrontendAdmin:       ctx.CopyFrontendAdmin,
		WriteFrontendEnvExample: ctx.WriteFrontendEnv,
	})
}
