package python

import (
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

type BuildEmitter interface {
	EmitPythonConfig(cfg *normalizer.ConfigDef) error
	EmitPythonRBAC(rbac *normalizer.RBACDef) error
	EmitPythonAuthStores(auth *normalizer.AuthDef) error
	EmitPythonFastAPIBackendFromIR(schema *ir.Schema, fallbackVersion string) error
}

type RegisterInput struct {
	Em       BuildEmitter
	IRSchema *ir.Schema
	CfgDef   *normalizer.ConfigDef
	AuthDef  *normalizer.AuthDef
	RBACDef  *normalizer.RBACDef
}

func Register(registry *generator.StepRegistry, in RegisterInput) {
	registry.Register(generator.Step{
		Name:     "Python Config",
		Requires: []compiler.Capability{compiler.CapabilityProfilePythonFastAPI},
		Run: func() error {
			return in.Em.EmitPythonConfig(in.CfgDef)
		},
	})
	registry.Register(generator.Step{
		Name:     "Python RBAC",
		Requires: []compiler.Capability{compiler.CapabilityProfilePythonFastAPI},
		Run: func() error {
			return in.Em.EmitPythonRBAC(in.RBACDef)
		},
	})
	registry.Register(generator.Step{
		Name: "Python Auth Stores",
		Requires: []compiler.Capability{
			compiler.CapabilityProfilePythonFastAPI,
			compiler.CapabilityAuth,
		},
		Run: func() error {
			return in.Em.EmitPythonAuthStores(in.AuthDef)
		},
	})
	registry.Register(generator.Step{
		Name: "Python FastAPI Backend",
		Requires: []compiler.Capability{
			compiler.CapabilityProfilePythonFastAPI,
			compiler.CapabilityHTTP,
			compiler.CapabilitySQLRepo,
		},
		Run: func() error {
			return in.Em.EmitPythonFastAPIBackendFromIR(in.IRSchema, compiler.Version)
		},
	})
}
