package compiler

import (
	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

// IRPhaseInput is the explicit contract accepted by the IR phase.
type IRPhaseInput struct {
	Normalized  NormalizePhaseOutput
	Config      normalizer.ConfigDef
	Auth        *normalizer.AuthDef
	RBAC        *normalizer.RBACDef
	Views       []normalizer.ViewDef
	Project     normalizer.ProjectDef
	InfraValues map[string]any
	Templates   []normalizer.TemplateDef
	BasePath    string
}

// IRPhaseOutput is the explicit contract produced by the IR phase.
type IRPhaseOutput struct {
	Schema *ir.Schema
}

// CompileForEmitOptions configures IR conversion after semantic phases.
type CompileForEmitOptions struct {
	Config      normalizer.ConfigDef
	Auth        *normalizer.AuthDef
	RBAC        *normalizer.RBACDef
	Views       []normalizer.ViewDef
	Project     normalizer.ProjectDef
	InfraValues map[string]any
	Templates   []normalizer.TemplateDef
}

// CompileForEmitOutput contains semantic phase output and emit-ready IR.
type CompileForEmitOutput struct {
	Normalized NormalizePhaseOutput
	IR         *ir.Schema
}

// RunIRPhase converts normalized model to IR, attaches infrastructure metadata,
// and performs IR-level semantic/dependency validation required by emitters.
func RunIRPhase(in IRPhaseInput) (IRPhaseOutput, error) {
	schema, err := ConvertAndTransform(
		in.Normalized.Entities,
		in.Normalized.Services,
		in.Normalized.Events,
		in.Normalized.Errors,
		in.Normalized.Endpoints,
		in.Normalized.Scopes,
		in.Normalized.Repos,
		in.Config,
		in.Auth,
		in.RBAC,
		in.Normalized.Schedules,
		in.Views,
		in.Project,
	)
	if err != nil {
		return IRPhaseOutput{}, WrapContractError(StageIR, ErrCodeIRConvertTransform, "convert and transform", err)
	}

	AttachNotificationInfra(
		schema,
		normalizer.InfraNotificationChannels(in.InfraValues),
		normalizer.InfraNotificationPolicies(in.InfraValues),
	)
	AttachTemplates(schema, in.Templates, in.BasePath)
	if err := ValidateIRSemantics(schema); err != nil {
		return IRPhaseOutput{}, WrapContractError(StageIR, ErrCodeIRSemanticValidate, "validate IR semantics", err)
	}
	if err := ValidateIRServiceDependencies(schema.Services); err != nil {
		return IRPhaseOutput{}, WrapContractError(StageIR, ErrCodeIRServiceDependencies, "validate service dependencies", err)
	}
	schema.Services = OrderIRServicesByDependencies(schema.Services)

	return IRPhaseOutput{Schema: schema}, nil
}

// CompileForEmit runs semantic phases (parse/normalize/flowsem) and IR phase,
// returning both normalized contracts and emit-ready IR.
func CompileForEmit(basePath string, pipelineOpts PipelineOptions, opts CompileForEmitOptions) (CompileForEmitOutput, error) {
	normalized, err := RunSemanticPhasesWithOptions(basePath, pipelineOpts)
	if err != nil {
		return CompileForEmitOutput{}, err
	}
	irOut, err := RunIRPhase(IRPhaseInput{
		Normalized:  normalized,
		Config:      opts.Config,
		Auth:        opts.Auth,
		RBAC:        opts.RBAC,
		Views:       opts.Views,
		Project:     opts.Project,
		InfraValues: opts.InfraValues,
		Templates:   opts.Templates,
		BasePath:    basePath,
	})
	if err != nil {
		return CompileForEmitOutput{}, err
	}
	return CompileForEmitOutput{
		Normalized: normalized,
		IR:         irOut.Schema,
	}, nil
}
