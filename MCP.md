# ANG Model Context Protocol (MCP) Server

The ANG compiler includes a built-in MCP server that allows AI agents (like Cursor, Claude Desktop, or IDE plugins) to interact with the compiler directly using structured data.

## Features

- **Structured Diagnostics**: `ang_validate` returns machine-readable violations with CUE paths and suggested fixes.
- **Project Locking**: Thread-safe operations for multiple projects.
- **Segmented IR**: Access specific parts of the system (Entities, Services) via `resource://ang/ir/...`.
- **Generative Prompts**: Interactive templates for creating new entities and CRUD operations.
- **Safe Apply**: `ang_apply` tool for applying changes with dry-run support and path validation.

## Usage

Start the MCP server using standard I/O:

```bash
ang mcp
```

## Tools

| Tool | Description |
|------|-------------|
| `ang_capabilities` | Returns compiler version and supported features. |
| `ang_validate` | Validates CUE intent and returns structured violations. |
| `ang_build` | Triggers full code generation. |
| `ang_graph` | Returns architecture graph data (Mermaid context). |
| `ang_apply` | Applies structural changes to CUE files (Dry-run by default). |

## Resources

| URI | Description |
|-----|-------------|
| `resource://ang/manifest` | Current system manifest (JSON). |
| `resource://ang/ir` | Full Intermediate Representation (JSON). |
| `resource://ang/ir/entities` | Only Entity definitions from IR. |
| `resource://ang/ir/services` | Only Service definitions from IR. |
| `resource://ang/diagnostics/latest` | Latest validation errors grouped by file. |

## Walkthroughs

### 1. Add Entity + CRUD End-to-End
A typical workflow for an AI agent creating a new feature:

1. **Prompt**: Use `add-entity(name="Order", fields="id:string,amount:int")` to get the CUE snippet and suggested file path.
2. **Safe Apply**: Call `ang_apply(file="cue/domain/order.cue", op="create", text="...", dry_run=false)` to create the file.
3. **Validate**: Call `ang_validate()` to ensure the project state is consistent.
4. **Build**: Call `ang_build()` to generate Go code and SQL migrations.

### 2. Fix Violations Loop
How an agent handles architectural or syntax errors:

1. **Validate**: Call `ang_validate()` and receive a violation like `ARCHITECTURE_VIOLATION` at `cue/api/posts.cue`.
2. **Inspect Fix**: Check `suggested_fix` field in the violation object.
3. **Apply Fix**: Use `ang_apply` with the suggested text and path.
4. **Verify**: Run `ang_validate()` again to confirm the fix works.

## Security

- **Path Pinning**: All file operations are restricted to the current workspace.
- **Extension Allowlist**: Only `.cue`, `.yaml`, `.json`, and `.md` files can be modified via `ang_apply`.
- **Metadata**: All tool responses include `schema_version` and `project_hash` for consistency checks.