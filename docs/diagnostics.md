# ANG diagnostics

## Diagnostic codes

Every structured diagnostic contains a stable `code`, source location when
available, a `docsURL`, and machine-applicable `suggestedFix` entries when ANG
can determine a safe replacement.

Use the code directly:

```bash
ang doctor --code DTO_FIELD_UNKNOWN
ang build --log-format json
ang validate --json
```

Code families:

- `CUE_*`: intent loading, normalization, and policy validation. Fix the
  referenced source below `cue/`, then run `ang validate`.
- `DTO_FIELD_UNKNOWN`: a Go selector does not exist in the normalized DTO.
  Apply the `before`/`after` replacement from `suggestedFix`.
- `IR_*`: canonical IR conversion, migration, or semantic validation.
- `TRANSFORMER_*` and `HOOK_*`: extension execution failures.
- `EMITTER_*`: capability resolution, output validation, or generation failure.

`ang doctor --code <CODE>` is the canonical source for code-specific guidance;
it is generated from the same registry used by the compiler and CLI.
