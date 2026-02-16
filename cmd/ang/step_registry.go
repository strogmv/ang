package main

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/targets"
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
	emailTemplates   []normalizer.EmailTemplateDef
	projectDef       *normalizer.ProjectDef
	targetOutput     OutputOptions
	pythonSDKEnabled bool
	isMicroservice   bool
}

func buildStepRegistry(in buildStepRegistryInput) (*generator.StepRegistry, []string, error) {
	registry := generator.NewStepRegistry()
	ctx := targets.BuildContext{
		Emitter:          in.em,
		IRSchema:         in.irSchema,
		MainContext:      in.ctx,
		Scenarios:        in.scenarios,
		Config:           in.cfgDef,
		Auth:             in.authDef,
		RBAC:             in.rbacDef,
		InfraValues:      in.infraValues,
		EmailTemplates:   in.emailTemplates,
		Project:          in.projectDef,
		PythonSDKEnabled: in.pythonSDKEnabled,
		IsMicroservice:   in.isMicroservice,
		TestStubsEnabled: in.targetOutput.TestStubs,
		ResolveMissingTests: func() ([]normalizer.Endpoint, error) {
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
		WriteFrontendEnv: func() error {
			return writeEnvExample(in.targetOutput)
		},
	}
	plugins, err := targets.ResolvePlugins(in.projectDef)
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		names = append(names, plugin.Name())
		plugin.RegisterSteps(registry, ctx)
	}
	return registry, names, nil
}

func joinPluginNames(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ",")
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
