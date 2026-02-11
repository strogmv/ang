package compiler

const (
	// CUE stage
	ErrCodeCUEDomainLoad           = "CUE_DOMAIN_LOAD_ERROR"
	ErrCodeCUEArchLoad             = "CUE_ARCH_LOAD_ERROR"
	ErrCodeCUEAPILoad              = "CUE_API_LOAD_ERROR"
	ErrCodeCUEEntityNormalize      = "CUE_ENTITY_NORMALIZE_ERROR"
	ErrCodeCUEServiceNormalize     = "CUE_SERVICE_NORMALIZE_ERROR"
	ErrCodeCUEEndpointNormalize    = "CUE_ENDPOINT_NORMALIZE_ERROR"
	ErrCodeCUERepoNormalize        = "CUE_REPO_NORMALIZE_ERROR"
	ErrCodeCUEScheduleNormalize    = "CUE_SCHEDULE_NORMALIZE_ERROR"
	ErrCodeCUEPipeline             = "CUE_PIPELINE_ERROR"
	ErrCodeCUEInfraLoad            = "CUE_INFRA_LOAD_ERROR"
	ErrCodeCUEInfraConfigParse     = "CUE_INFRA_CONFIG_PARSE_ERROR"
	ErrCodeCUEInfraAuthParse       = "CUE_INFRA_AUTH_PARSE_ERROR"
	ErrCodeCUERBACLoad             = "CUE_RBAC_LOAD_ERROR"
	ErrCodeCUERBACParse            = "CUE_RBAC_PARSE_ERROR"
	ErrCodeCUEPoliciesLoad         = "CUE_POLICIES_LOAD_ERROR"
	ErrCodeCUEPoliciesParse        = "CUE_POLICIES_PARSE_ERROR"
	ErrCodeCUEViewsLoad            = "CUE_VIEWS_LOAD_ERROR"
	ErrCodeCUEViewsParse           = "CUE_VIEWS_PARSE_ERROR"
	ErrCodeCUEProjectLoad          = "CUE_PROJECT_LOAD_ERROR"
	ErrCodeCUEProjectParse         = "CUE_PROJECT_PARSE_ERROR"
	ErrCodeCUETargetsParse         = "CUE_TARGETS_PARSE_ERROR"
	ErrCodeCUELintLoad             = "CUE_LINT_LOAD_ERROR"
	ErrCodeCUEPolicyValidate       = "CUE_POLICY_VALIDATE_ERROR"
	ErrCodeCUEPolicyLoad           = "CUE_POLICY_LOAD_ERROR"
	ErrCodeCUETestCoveragePipeline = "CUE_TEST_COVERAGE_PIPELINE_ERROR"

	// IR stage
	ErrCodeIRConvertTransform    = "IR_CONVERT_TRANSFORM_ERROR"
	ErrCodeIRServiceDependencies = "IR_SERVICE_DEPENDENCY_ERROR"
	ErrCodeIRVersionMigration    = "IR_VERSION_MIGRATION_ERROR"
	ErrCodeIRSemanticValidate    = "IR_SEMANTIC_VALIDATE_ERROR"

	// Transformers stage
	ErrCodeTransformerApply = "TRANSFORMER_APPLY_ERROR"
	ErrCodeHookProcess      = "HOOK_PROCESS_ERROR"

	// Emitters stage
	ErrCodeEmitterOptions           = "EMITTER_OPTIONS_ERROR"
	ErrCodeEmitterStep              = "EMITTER_STEP_ERROR"
	ErrCodeEmitterMCPGen            = "EMITTER_MCP_GENERATION_ERROR"
	ErrCodeEmitterCapabilityResolve = "EMITTER_CAPABILITY_RESOLVE_ERROR"
)

// StableErrorCodes is the canonical registry of compiler/CLI stage error codes.
var StableErrorCodes = []string{
	ErrCodeCUEDomainLoad,
	ErrCodeCUEArchLoad,
	ErrCodeCUEAPILoad,
	ErrCodeCUEEntityNormalize,
	ErrCodeCUEServiceNormalize,
	ErrCodeCUEEndpointNormalize,
	ErrCodeCUERepoNormalize,
	ErrCodeCUEScheduleNormalize,
	ErrCodeCUEPipeline,
	ErrCodeCUEInfraLoad,
	ErrCodeCUEInfraConfigParse,
	ErrCodeCUEInfraAuthParse,
	ErrCodeCUERBACLoad,
	ErrCodeCUERBACParse,
	ErrCodeCUEPoliciesLoad,
	ErrCodeCUEPoliciesParse,
	ErrCodeCUEViewsLoad,
	ErrCodeCUEViewsParse,
	ErrCodeCUEProjectLoad,
	ErrCodeCUEProjectParse,
	ErrCodeCUETargetsParse,
	ErrCodeCUELintLoad,
	ErrCodeCUEPolicyValidate,
	ErrCodeCUEPolicyLoad,
	ErrCodeCUETestCoveragePipeline,
	ErrCodeIRConvertTransform,
	ErrCodeIRServiceDependencies,
	ErrCodeIRVersionMigration,
	ErrCodeIRSemanticValidate,
	ErrCodeTransformerApply,
	ErrCodeHookProcess,
	ErrCodeEmitterOptions,
	ErrCodeEmitterStep,
	ErrCodeEmitterMCPGen,
	ErrCodeEmitterCapabilityResolve,
}
