# ANG MCP Server

ANG provides an MCP (Model Context Protocol) server to integrate with AI agents like Claude Desktop or Cursor.

## How to run

```bash
ang mcp
```

## Available Tools

### `ang_validate`
Validates the CUE structure and architectural rules of an ANG project.
- **Arguments:**
  - `project_path` (string, optional): Path to the project root. Defaults to current directory.

### `ang_build`
Generates Go code, SQL migrations, and TypeScript SDK from CUE intent.
- **Arguments:**
  - `project_path` (string, optional): Path to the project root.

### `ang_graph`
Generates a Mermaid architecture diagram.
- **Arguments:**
  - `project_path` (string, optional): Path to the project root.

## Available Resources

- `resource://ang/manifest`: Returns the content of `ang-manifest.json`.
- `resource://ang/project/summary`: Returns a short text summary of the project.

## Available Prompts

- `add-entity`: Instructions on how to add a new domain entity.
- `add-crud`: Instructions on how to add CRUD operations for an entity.
