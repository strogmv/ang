package python

import (
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter/contracts"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/ir"
)

type BuildEmitter interface {
	EmitPythonConfig(cfg *contracts.ConfigDef) error
	EmitPythonRBAC(rbac *contracts.RBACDef) error
	EmitPythonAuthStores(auth *contracts.AuthDef) error
	EmitPythonFastAPIBackendFromIR(schema *ir.Schema, fallbackVersion string) error
}

type RegisterInput struct {
	Em          BuildEmitter
	IRSchema    *ir.Schema
	CfgDef      *contracts.ConfigDef
	AuthDef     *contracts.AuthDef
	RBACDef     *contracts.RBACDef
	InfraValues map[string]any
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
	registerInfraPythonSteps(registry, in)
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
