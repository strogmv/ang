package emitter

import (
	"fmt"
	"strings"

	pyemitter "github.com/strogmv/ang/compiler/emitter/python"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// EmitPythonSDK generates a minimal Python client SDK from normalized endpoints.
func (e *Emitter) EmitPythonSDK(endpoints []normalizer.Endpoint, services []normalizer.Service, entities []normalizer.Entity, project *normalizer.ProjectDef) error {
	version := strings.TrimSpace(e.Version)
	if project != nil {
		if v := strings.TrimSpace(project.Version); v != "" {
			version = v
		}
	}
	if version == "" {
		version = "0.1.0"
	}
	rt := pyemitter.Runtime{
		Funcs:        e.getSharedFuncMap(),
		ReadTemplate: ReadTemplateByPath,
	}
	return pyemitter.EmitSDK(e.OutputDir, version, rt, endpoints, services, entities)
}

func (e *Emitter) EmitPythonSDKFromIR(schema *ir.Schema, fallbackVersion string) error {
	if err := ir.MigrateToCurrent(schema); err != nil {
		return fmt.Errorf("migrate ir schema: %w", err)
	}
	rt := pyemitter.Runtime{
		Funcs:        e.getSharedFuncMap(),
		ReadTemplate: ReadTemplateByPath,
	}
	return pyemitter.EmitSDKFromIR(e.OutputDir, fallbackVersion, rt, schema)
}
