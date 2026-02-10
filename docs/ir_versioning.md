# IR Versioning Policy

`ir.Schema` is a versioned contract and the core compatibility boundary of ANG.

## Current Version

- Current version: `1`
- Field: `ir.Schema.IRVersion` (`json:"ir_version"`)

## Compatibility Rules

1. Any IR shape change must increment/adjust version handling.
2. Direct breaking changes are forbidden without migration adapters.
3. Legacy schemas must be upgraded via adapters before transformers/emitters run.

## Adapter Entry Point

- `ir.MigrateToCurrent(schema *ir.Schema) error`

Behavior:

- Empty version (`""`) is treated as legacy `v0` and migrated to `v1`.
- Unknown version returns an explicit error.

## Enforcement Points

- `compiler.ConvertAndTransform` calls migration before transformer stage.
- `transformers.Registry.Apply` and `HookRegistry.Process` call migration defensively.
- `emitter.EmitFromIR` calls migration before generation.

This guarantees no hidden schema drift between stages.

