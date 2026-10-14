# Error Codes Registry

This file documents stage-scoped stable error codes used by ANG compiler/CLI.

Canonical source in code:
- `compiler/error_codes.go`
- `compiler/error_codes_test.go` (uniqueness guard)

## CUE

Every `*_LOAD_ERROR` of a CUE folder means a file there does not parse or the package does not
evaluate; the message names `file:line:col`. A folder is loaded strictly: no file is skipped.

- `CUE_DOMAIN_LOAD_ERROR`
- `CUE_ARCH_LOAD_ERROR`
- `CUE_API_LOAD_ERROR`
- `CUE_ENTITY_NORMALIZE_ERROR`
- `CUE_SERVICE_NORMALIZE_ERROR`
- `CUE_ENDPOINT_NORMALIZE_ERROR`
- `CUE_REPO_NORMALIZE_ERROR`
- `CUE_REPO_LOAD_ERROR` — `cue/repo` does not load; the line also names the finder
- `CUE_EVENTS_LOAD_ERROR` — `cue/events` does not load, or an event in it does not decode
- `CUE_ERRORS_LOAD_ERROR` — `cue/errors` does not load, or an entry of `#Errors` does not decode (`code: "E404"`)
- `CUE_PROJECTIONS_LOAD_ERROR` — `cue/projections` does not load
- `CUE_SCHEDULE_NORMALIZE_ERROR`
- `CUE_PIPELINE_ERROR`
- `CUE_INFRA_LOAD_ERROR`
- `CUE_INFRA_CONFIG_PARSE_ERROR`
- `CUE_INFRA_AUTH_PARSE_ERROR`
- `CUE_RBAC_LOAD_ERROR`
- `CUE_RBAC_PARSE_ERROR`
- `CUE_POLICIES_LOAD_ERROR`
- `CUE_POLICIES_PARSE_ERROR`
- `CUE_VIEWS_LOAD_ERROR`
- `CUE_VIEWS_PARSE_ERROR`
- `CUE_PROJECT_LOAD_ERROR` — `cue/project` does not load
- `CUE_PROJECT_PARSE_ERROR`
- `CUE_TARGETS_PARSE_ERROR`
- `CUE_LINT_LOAD_ERROR`
- `CUE_POLICY_VALIDATE_ERROR`
- `CUE_POLICY_LOAD_ERROR` — `cue/policy` (or the legacy `cue/policies`) does not load
- `CUE_TEST_COVERAGE_PIPELINE_ERROR`
- `RAW_GO_IMPORT_ERROR` — an `imports:` entry of raw Go (`impls: go:` or a `logic.Call`) that the code does not use, a package an `impls: go:` block uses without listing it, or a `logic.Call` lambda using `name.Sel` that two standard packages export (`rand.Read`: crypto/rand and math/rand) with nothing importing it; one line per problem: `file:line: Service.Method: …`

## IR

- `IR_CONVERT_TRANSFORM_ERROR`
- `IR_SERVICE_DEPENDENCY_ERROR`
- `IR_VERSION_MIGRATION_ERROR`

## Transformers

- `TRANSFORMER_APPLY_ERROR`
- `HOOK_PROCESS_ERROR`

## Emitters

- `EMITTER_OPTIONS_ERROR`
- `EMITTER_STEP_ERROR`
- `ANG_LOCK_MISMATCH` — the running binary is not the ang revision pinned in the project's `ang.lock`; generation stops before anything is written unless `--allow-lock-mismatch` is passed
- `EMITTER_MCP_GENERATION_ERROR`
- `EMITTER_CAPABILITY_RESOLVE_ERROR`
