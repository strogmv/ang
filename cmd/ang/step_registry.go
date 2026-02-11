package main

import (
	"github.com/strogmv/ang/compiler/emitter"
	goemitter "github.com/strogmv/ang/compiler/emitter/go"
	pyemitter "github.com/strogmv/ang/compiler/emitter/python"
	sharedsteps "github.com/strogmv/ang/compiler/emitter/shared"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

type buildStepRegistryInput struct {
	em               *emitter.Emitter
	irSchema         *ir.Schema
	ctx              emitter.MainContext
	scenarios        []normalizer.ScenarioDef
	cfgDef           *normalizer.ConfigDef
	authDef          *normalizer.AuthDef
	rbacDef          *normalizer.RBACDef
	infraValues      map[string]any
	projectDef       *normalizer.ProjectDef
	targetOutput     OutputOptions
	pythonSDKEnabled bool
	isMicroservice   bool
}

func buildStepRegistry(in buildStepRegistryInput) *generator.StepRegistry {
	registry := generator.NewStepRegistry()

	sharedsteps.Register(registry, sharedsteps.RegisterInput{
		Em:               in.em,
		IRSchema:         in.irSchema,
		ProjectDef:       in.projectDef,
		PythonSDKEnabled: in.pythonSDKEnabled,
	})

	pyemitter.Register(registry, pyemitter.RegisterInput{
		Em:          in.em,
		IRSchema:    in.irSchema,
		CfgDef:      in.cfgDef,
		AuthDef:     in.authDef,
		RBACDef:     in.rbacDef,
		InfraValues: in.infraValues,
	})

	goemitter.Register(registry, goemitter.RegisterInput{
		Em:               in.em,
		IRSchema:         in.irSchema,
		Ctx:              in.ctx,
		Scenarios:        in.scenarios,
		CfgDef:           in.cfgDef,
		AuthDef:          in.authDef,
		RBACDef:          in.rbacDef,
		InfraValues:      in.infraValues,
		IsMicroservice:   in.isMicroservice,
		TestStubsEnabled: in.targetOutput.TestStubs,
		ResolveMissingTestStubs: func() ([]normalizer.Endpoint, error) {
			endpoints := coverageEndpointsFromIR(in.irSchema.Endpoints)
			report, err := checkTestCoverage(endpoints, "tests")
			if err != nil {
				return nil, err
			}
			return missingEndpointsFromCoverage(endpoints, report.MissingTests), nil
		},
		CopyFrontendSDK: func() error {
			return copyFrontendSDK(in.targetOutput.FrontendDir, in.targetOutput.FrontendAppDir)
		},
		CopyFrontendAdmin: func() error {
			return copyFrontendAdmin(in.targetOutput.FrontendAdminDir, in.targetOutput.FrontendAdminAppDir)
		},
		WriteFrontendEnvExample: func() error {
			return writeEnvExample(in.targetOutput)
		},
	})

	return registry
}

func coverageEndpointsFromIR(endpoints []ir.Endpoint) []normalizer.Endpoint {
	out := make([]normalizer.Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		n := normalizer.Endpoint{
			Method:      ep.Method,
			Path:        ep.Path,
			ServiceName: ep.Service,
			RPC:         ep.RPC,
		}
		if ep.Auth != nil {
			n.AuthType = ep.Auth.Type
		}
		if ep.TestHints != nil {
			n.TestHints = &normalizer.TestHints{
				HappyPath:  ep.TestHints.HappyPath,
				ErrorCases: append([]string{}, ep.TestHints.ErrorCases...),
			}
		}
		out = append(out, n)
	}
	return out
}

func missingEndpointsFromCoverage(endpoints []normalizer.Endpoint, missingTests []EndpointCoverage) []normalizer.Endpoint {
	if len(missingTests) == 0 {
		return nil
	}
	missingMap := make(map[string]bool, len(missingTests))
	for _, m := range missingTests {
		missingMap[m.Method+" "+m.Path] = true
	}
	var missing []normalizer.Endpoint
	for _, ep := range endpoints {
		if missingMap[ep.Method+" "+ep.Path] {
			missing = append(missing, ep)
		}
	}
	return missing
}
