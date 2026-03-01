# ANG Architecture

ANG is an intent compiler.  
The authoritative pipeline is:

`CUE -> Parser -> Normalizer -> FlowSem -> IR -> Emitters -> Generated Artifacts`

## High-Level Stages

## 1. Parser

Package:

- `compiler/parser`

Responsibility:

- Load CUE domains and project state.
- Produce typed CUE values for downstream normalization.

## 2. Normalizer

Package:

- `compiler/normalizer`

Responsibility:

- Convert raw CUE structures into normalized domain/service/endpoint/config models.
- Resolve operation contracts and metadata.
- Attach source positions and diagnostics context.

## 3. Flow Semantic Validation

Package:

- `compiler/flowsem`

Responsibility:

- Validate flow actions and required arguments/branches.
- Enforce action-specific constraints before codegen.
- Surface deterministic diagnostics with CUE location.

## 4. IR Conversion and Transform

Packages:

- `compiler/ir`
- `compiler/transformers`

Responsibility:

- Build language-neutral IR (`ir.Schema`).
- Apply deterministic transformers and migration adapters.

## 5. Emitters

Package:

- `compiler/emitter`

Responsibility:

- Generate backend, SDK, API specs, DB artifacts, and deployment files.
- Render flow into runtime code (including control/resilience actions).

Recent structure:

- Flow codegen is split by concern (`service_flow_codegen_*`) instead of a single monolith.

## 6. CLI Orchestration

Package:

- `cmd/ang`

Responsibility:

- User command surface (`init`, `build`, `lint`, `doctor`, `up`, `smoke`, etc.).
- Build phases (`all|plan|apply`), startup preflight, and diagnostics entry points.

## Source of Truth Model

- Intent lives in CUE (`cue/`, `cue.mod/`).
- Generated runtime artifacts are outputs, not hand-edited sources.

If generated code is wrong, fix:

1. CUE intent,
2. normalizer/flowsem contract,
3. emitter logic,

not the generated file itself.

## Extension Points

- New flow actions: `flowsem` contract + emitter implementation + tests.
- New target behavior: emitter/template modules.
- New diagnostics: normalizer/flowsem checks with stable error codes.

## Stability Contracts

- Deterministic generation for same input.
- Build report with stage/codes for failure isolation.
- IR/schema version compatibility checks.

See also:

- [Compiler Contract](./compiler_contract.md)
- [Planner Contract](./planner_contract.md)
- [IR Versioning](./ir_versioning.md)
