# Improvement Backlog (Engineering Quality)

This is a living list of quality upgrades that improve generated-code reliability and maintainability.

## Priority A: Generator Safety

- Continue replacing string-based codegen with AST-based emission in flow-heavy paths.
- Add golden tests for every new flow action contract and edge-case branch.
- Add deterministic variable naming tests for nested control-flow and retries.

## Priority A: Type Strictness

- Reduce `any` usage in normalized flow args where concrete types are known.
- Move runtime shape checks into compile-time validation where possible.
- Tighten cross-checks between action args and repository/service signatures.

## Priority B: DX Hardening

- Expand `ang doctor start` with optional dependency checks for Atlas/Node when relevant.
- Add `ang up --frontend` workflow for SDK/frontend projects.
- Add auto-generated runbook snippet to project root after `ang init`.

## Priority B: Observability

- Emit structured diagnostics IDs consistently across normalizer/flowsem/emitter.
- Generate troubleshooting links from error codes to docs.
- Add `ang doctor --code <ERR_CODE>` shortcut for instant guidance.

## Priority C: Performance & Scale

- Benchmark large-flow compile and emission latency.
- Introduce parallel emission where outputs are independent.
- Add cache invalidation tests for incremental build plans.

## Priority C: Documentation

- Keep command docs synchronized with CLI flag changes per release.
- Add minimal “copy/paste recipes” per common use case (auth service, event worker, webhook).
- Publish a short “migration notes” file for each breaking behavior change.
