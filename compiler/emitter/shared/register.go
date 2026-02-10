package sharedsteps

import (
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

type RegisterInput struct {
	Em               *emitter.Emitter
	IRSchema         *ir.Schema
	Endpoints        []normalizer.Endpoint
	Services         []normalizer.Service
	Events           []normalizer.EventDef
	BizErrors        []normalizer.ErrorDef
	ProjectDef       *normalizer.ProjectDef
	PythonSDKEnabled bool
}

func Register(registry *generator.StepRegistry, in RegisterInput) {
	registry.Register(generator.Step{
		Name:     "OpenAPI",
		Requires: []compiler.Capability{compiler.CapabilityHTTP},
		Run: func() error {
			return in.Em.EmitOpenAPI(in.Endpoints, in.Services, in.BizErrors, in.ProjectDef)
		},
	})
	registry.Register(generator.Step{
		Name:     "AsyncAPI",
		Requires: []compiler.Capability{compiler.CapabilityEvents},
		Run: func() error {
			return in.Em.EmitAsyncAPI(in.Events, in.ProjectDef)
		},
	})
	registry.Register(generator.Step{
		Name:     "Python SDK",
		Requires: []compiler.Capability{compiler.CapabilityHTTP},
		Run: func() error {
			if !in.PythonSDKEnabled {
				return nil
			}
			return in.Em.EmitPythonSDKFromIR(in.IRSchema, compiler.Version)
		},
	})
	registry.Register(generator.Step{
		Name: "System Manifest",
		Run: func() error {
			return in.Em.EmitManifest(in.IRSchema)
		},
	})
}
