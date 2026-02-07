# ANG Project Context

AI-Friendly Intent Compiler for Go Backends.

## Core Policy
**Agent writes ONLY CUE code in `cue/` folder.**
**Generated code in `internal/`, `api/`, `sdk/` is READ-ONLY for AI.**

## Development Commands
- `ang build` - Standard generation (use via `ang_generate` tool in MCP)
- `ang validate` - Check CUE integrity
- `go test ./...` - Run tests

## MCP Tools
This project supports Model Context Protocol. Run `ang mcp` to start the server.
Available tools:
- `cue_apply_patch`: Edit CUE intent
- `ang_generate`: Rebuild the whole project from CUE
- `repo_diff`: Token-efficient way to see changes in Go code

## Code Style
- CUE: Strict schema validation via `cue/schema`
- Go: Standard library preferred, clean architecture (ports and adapters)
