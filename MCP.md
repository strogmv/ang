# ANG Model Context Protocol (MCP) Server

The ANG compiler includes a built-in MCP server that enforces a strict **Intent-First** development policy.

## Core Policy

> **Agent writes only CUE. ANG writes code. Agent reads code and runs tests.**

### Repository Zones
1. `cue/**` — **Read/Write**. The agent edits CUE intent here.
2. Everything else in a generated project — **Read-Only**. Generated Go, SQL and manifests are read, never edited directly.

## Tools

`ang_list_tools` prints every registered tool. The ones an agent uses most:

### Session and schema
- `ang_mcp_health`: profile, limits, bootstrap state (bootstrap-exempt).
- `ang_schema`: the operation schema for AI; `ang_ops_context`: schema + actions + diagnostics + proof in one call.
- `ang_status`, `ang_snapshot`: project state.

### CUE intent (write)
- `cue_apply_patch`, `cue_set_field`, `cue_add_endpoint`: structured CUE edits (restricted to `cue/`).
- `cue_history`, `cue_undo`: edit history and rollback.

### Planning and generation
- `ang_plan`, `ang_do`: plan and apply an intent ("fix: …", "add: …").
- `ang_validate`, `ang_validate_logic`: validate CUE / a Go snippet before inserting it.
- `ang_dry_run`: validate and return the diff a build would make, writing nothing.
- `run_preset`: `build` (runs `ang build`), `unit` (go test), `lint`.

### Analysis (read-only)
- `ang_search`, `repo_read_symbol`: find and read code without loading whole files.
- `ang_doctor`, `ang_explain_error`: diagnose a failed build, explain a diagnostic code.
- `ang_event_map`, `ang_rbac_inspector`, `ang_diff_architecture`, `ang_model_diff`, `ang_template_diff`.

## Resources

| URI | Description |
|-----|-------------|
| `resource://ang/ir` | Full Intermediate Representation (JSON). |
| `resource://ang/ai_hints` | Patterns and rules for LLM context optimization. |
| `resource://ang/transformers` | Catalog of available extensions (Tracing, Caching, Security). |
| `resource://ang/diagnostics/latest` | Latest validation errors grouped by file. |

## Walkthrough: The Loop

1. **Modify**: edit `cue/domain/user.cue` via `cue_apply_patch`.
2. **Preview**: `ang_dry_run` shows the generated diff without writing.
3. **Generate**: `run_preset('build')`.
4. **Verify**: `run_preset('unit')`.
5. **Iterate**: if anything fails, `ang_doctor` / `ang_explain_error`, then back to step 1 (editing CUE).

## Response Format Compatibility

ANG MCP now supports a unified tool response envelope (default on) and a compatibility mode.

- Default behavior (`ANG_MCP_ENVELOPE` unset): MCP tools return unified JSON with `tool`, `status`, `active_profile`, and `payload`.
- Compatibility mode: set `ANG_MCP_ENVELOPE=off` (also supports `0`, `false`, `no`) to return legacy/raw tool responses.

Examples:

```bash
# Unified envelope (default)
ang mcp

# Legacy compatibility mode
ANG_MCP_ENVELOPE=off ang mcp
```

## Health Check Tool

Use `ang_mcp_health` for quick MCP diagnostics. It is bootstrap-exempt and returns:

- current profile and effective limits,
- envelope status,
- runtime config path/error,
- effective workflows,
- bootstrap state and last action.

## Envelope Versioning

Unified MCP envelope now includes `schema_version` (default: `mcp-envelope/v1`).
This lets MCP clients migrate safely between future response format versions.
