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

## MCP-First (Mandatory)

Before any edits/tests, run this MCP bootstrap sequence:

1. `ang_mcp_health`
2. `ang_schema`
3. `ang_validate`

If MCP tools are available and this sequence is skipped, stop and explain why.
Prefer MCP tools (`ang_*`, `cue_*`) over direct file edits whenever possible.

Suggested workflow:
1. Run MCP bootstrap sequence above.
2. Read current intent in `cue/`.
3. Apply minimal CUE changes.
4. Run `ang validate`.
5. Run `ang build`.
6. Inspect generated diff and tests.
