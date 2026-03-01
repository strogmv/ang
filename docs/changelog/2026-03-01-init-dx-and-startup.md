# PR Changelog: Init DX + Startup Reliability

Date: 2026-03-01
Scope: `cmd/ang`, init templates, startup checks, tests

## Summary

This PR hardens first-run developer experience for generated projects:

- Adds a one-command local bootstrap flow (`ang up`) with preflight and smoke checks.
- Adds deterministic startup diagnostics (`ang doctor start`, `ang smoke`).
- Makes `ang init` scaffolds runnable without manual module bootstrapping (`go.mod` included).
- Adds optional task-runner workflow (`Taskfile.yml`) and Go workspace file (`go.work`) to generated projects.
- Documents MCP-first workflow in generated `AGENTS.md`/`README.md`.

## Problem

Users hit avoidable dead-ends after `ang init`:

- Missing runtime/module files blocked `ang build` self-checks.
- Startup prerequisites (tools/env/ports/compose) were not validated early.
- No standard command surface for beginners beyond raw command memorization.

## Changes

### CLI

- New `ang up` command:
  - Runs startup preflight (`doctor start`) unless skipped.
  - Brings up docker-compose when available.
  - Runs `ang build`.
  - Runs smoke checks (`/health`, `/health/ready`).
- Extended `ang doctor` with subcommand:
  - `ang doctor start` checks tools/files/config/compose/http port.
- New `ang smoke` command:
  - Verifies API readiness endpoints with configurable base URL and timeout.

### Init Scaffolding

- Template init now generates:
  - `go.mod` (if absent),
  - `.env.example`,
  - `Taskfile.yml`,
  - `go.work`,
  - `scripts/dev-*.sh`,
  - `Makefile`, `AGENTS.md`, `README.md`, CUE and DB starter files.
- Legacy minimal scaffold now also generates:
  - `go.mod`,
  - `go.work`,
  - `Taskfile.yml`.

### Template docs

- Generated README quickstart now includes:
  - `ang up`,
  - optional `task up`,
  - MCP-first bootstrap sequence for AI sessions.

## Test Coverage

- Added startup behavior tests in `cmd/ang/startup_test.go`:
  - compose command resolution and fallback,
  - config preflight behavior for missing required env,
  - HTTP port resolution priority,
  - smoke success/failure behavior.
- Expanded init tests:
  - `cmd/ang/init_templates_test.go`,
  - `cmd/ang/main_init_test.go`,
  to assert generation of `go.mod`, `go.work`, `Taskfile.yml`, `.env.example`, and bootstrap scripts.

## Compatibility and Risk

- Backward compatible command additions.
- Default startup behavior remains strict but allows overrides (`--strict=false`, skip flags).
- Generated files are additive and intended to reduce onboarding friction.

## Verification

- `go test ./cmd/ang -count=1` passes.
- `go test ./compiler/... -count=1` passes, including `compiler/ir`.
- End-to-end local flow verified:
  - `ang init --template saas ...`
  - `ang build` succeeds in generated project without manual `go.mod` creation.

## Follow-ups

- Add `ang up --frontend` (optional) to auto-run frontend install/dev flow when frontend is present.
- Add optional `ang init --minimal` profile for ultra-lean bootstrap.
