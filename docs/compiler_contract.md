# Compiler Contract (Draft)

ANG compiler has explicit stages:

1. `CUE` -> normalized model
2. `IR` -> canonical `ir.Schema`
3. `TRANSFORMERS` -> transformed `ir.Schema`
4. `EMITTERS` -> generated artifacts

Stage ownership is strict:

- `CUE`: parse + normalize only
- `IR`: semantic conversion only
- `TRANSFORMERS`: IR mutation only
- `EMITTERS`: filesystem generation only

Target selection strategy is capability-based (`docs/capability_matrix.md`), not language-specific `if/else`.

## Error Shape

Fatal errors should be typed as:

- `Stage`
- `Code`
- `Op`
- `Cause` (wrapped error)

Implemented via `compiler.ContractError`.
Stable code registry: `docs/error_codes.md` (`compiler/error_codes.go` is source of truth).

IR versioning and adapters: `docs/ir_versioning.md`.

Examples:

- `CUE_DOMAIN_LOAD_ERROR`
- `CUE_PROJECT_PARSE_ERROR`
- `IR_CONVERT_TRANSFORM_ERROR`
- `TRANSFORMER_APPLY_ERROR`
- `HOOK_PROCESS_ERROR`
- `EMITTER_STEP_ERROR`

## No Hidden Magic

- No implicit semantic side effects across stage boundaries.
- No template-level business logic.
- No silent semantic fallback without diagnostics.
