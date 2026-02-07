# ANG Model Context Protocol (MCP) Server

The ANG compiler includes a built-in MCP server that enforces a strict **Intent-First** development policy.

## Core Policy

> **Agent writes only CUE. ANG writes code. Agent reads code and runs tests.**

### Repository Zones
1. `/cue/**` — **Read/Write**. The agent is allowed to modify CUE intent files here.
2. `/**` (Other) — **Read-Only**. Generated Go code, SQL, and manifests can only be read, never modified directly by the agent.

## Tools

### CUE Intent (Write)
- `cue_read(path)`: Read CUE files from `/cue`.
- `cue_apply_patch(path, content)`: Update CUE file content (restricted to `/cue`).
- `cue_fmt(path)`: Format CUE files.

### Generation (ANG)
- `ang_generate()`: The **only** way to update the codebase. Runs `ang build` and returns a manifest of changed files.

### Code Analysis (Read-Only)
- `repo_read_file(path)`: Read generated code or artifacts.
- `repo_diff()`: Get a git-powered diff of changes (token-efficient).

### Runtime & Tests
- `run_tests(target)`: Runs `go test` and returns structured results.

## Resources

| URI | Description |
|-----|-------------|
| `resource://ang/ir` | Full Intermediate Representation (JSON). |
| `resource://ang/ai_hints` | Patterns and rules for LLM context optimization. |
| `resource://ang/transformers` | Catalog of available extensions (Tracing, Caching, Security). |
| `resource://ang/diagnostics/latest` | Latest validation errors grouped by file. |

## Walkthrough: The Loop

1. **Modify**: Agent edits `cue/domain/user.cue` via `cue_apply_patch`.
2. **Generate**: Agent calls `ang_generate()`.
3. **Inspect**: Agent calls `repo_diff()` to see how Go code changed.
4. **Verify**: Agent calls `run_tests()` to ensure the generated code works.
5. **Iterate**: If tests fail, agent goes back to step 1 (editing CUE).