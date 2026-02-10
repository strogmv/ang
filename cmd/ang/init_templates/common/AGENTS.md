# AGENTS.md

Rules for AI agents in this repository:

1. Edit intent only in `cue/` unless explicitly instructed otherwise.
2. Never edit generated code directly in:
   - `internal/`
   - `api/`
   - `sdk/`
   - `db/schema/`
   - `db/queries/`
3. To change generated behavior, update CUE intent and run build again.
4. If a bug appears in generated files, fix the CUE source that produced it.
5. Prefer deterministic, minimal changes and preserve unrelated user edits.

Suggested workflow:
1. Read current intent in `cue/`.
2. Apply minimal CUE changes.
3. Run `ang validate`.
4. Run `ang build`.
5. Inspect generated diff and tests.

