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
	entities         []normalizer.Entity
	services         []normalizer.Service
	endpoints        []normalizer.Endpoint
	repos            []normalizer.Repository
	events           []normalizer.EventDef
	bizErrors        []normalizer.ErrorDef
	schedules        []normalizer.ScheduleDef
	scenarios        []normalizer.ScenarioDef
	views            []normalizer.ViewDef
	cfgDef           *normalizer.ConfigDef
	authDef          *normalizer.AuthDef
	rbacDef          *normalizer.RBACDef
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
		Endpoints:        in.endpoints,
		Services:         in.services,
		Events:           in.events,
		BizErrors:        in.bizErrors,
		ProjectDef:       in.projectDef,
		PythonSDKEnabled: in.pythonSDKEnabled,
	})

	pyemitter.Register(registry, pyemitter.RegisterInput{
		Em:       in.em,
		IRSchema: in.irSchema,
		CfgDef:   in.cfgDef,
		AuthDef:  in.authDef,
		RBACDef:  in.rbacDef,
	})

	goemitter.Register(registry, goemitter.RegisterInput{
		Em:               in.em,
		IRSchema:         in.irSchema,
		Ctx:              in.ctx,
		Entities:         in.entities,
		Services:         in.services,
		Endpoints:        in.endpoints,
		Repos:            in.repos,
		Events:           in.events,
		BizErrors:        in.bizErrors,
		Schedules:        in.schedules,
		Scenarios:        in.scenarios,
		Views:            in.views,
		CfgDef:           in.cfgDef,
		AuthDef:          in.authDef,
		RBACDef:          in.rbacDef,
		IsMicroservice:   in.isMicroservice,
		TestStubsEnabled: in.targetOutput.TestStubs,
		ResolveMissingTestStubs: func() ([]normalizer.Endpoint, error) {
			report, err := checkTestCoverage(in.endpoints, "tests")
			if err != nil {
				return nil, err
			}
			return missingEndpointsFromCoverage(in.endpoints, report.MissingTests), nil
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
