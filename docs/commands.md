# ANG CLI Commands

This page lists practical commands and high-value flags.  
For command-specific flags, run `ang <command> --help` where supported.

## Project Bootstrap

### `ang init`

Initialize a new project scaffold.

```bash
ang init [dir] --template saas|ecommerce|marketplace --lang go --db postgres --module github.com/acme/app --force
```

Common flags:

- `--template`: starter blueprint
- `--module`: Go/CUE module path
- `--force`: allow writing into non-empty target directory

### `ang up`

One-command local startup.

```bash
ang up [--project-path .] [--compose-file docker-compose.yml] [--skip-doctor] [--skip-compose] [--skip-build] [--skip-smoke]
```

Common flags:

- `--doctor-strict=true|false`
- `--detach=true|false`

## Build & Validation

### `ang validate`

Validate CUE and architecture model.

### `ang lint`

Semantic linting and optional test-coverage checks.

```bash
ang lint [--json] [--check-test-coverage] [--test-dir tests] [--min-coverage 80] [--generate-stubs]
```

### `ang build`

Compile CUE intent into code and artifacts.

```bash
ang build [--mode in_place|release] [--backend-dir .] [--target go] [--dry-run] [--run-tests]
```

Planning mode:

```bash
ang build --phase plan --out-plan .ang/build.plan.json --json
ang build --phase apply --plan-file .ang/build.plan.json
```

## Startup Diagnostics

### `ang doctor`

Analyze build logs and suggest CUE fixes.

```bash
ang doctor [--log-file ang-build.log]
ang doctor --stdin < ang-build.log
ang doctor --log "inline text"
```

### `ang doctor start`

Preflight startup checks (tools, files, config, compose, ports).

```bash
ang doctor start [--project-path .] [--skip-config] [--strict=true|false]
```

### `ang smoke`

Health endpoints check (`/health`, `/health/ready`).

```bash
ang smoke [--base-url http://localhost:8080] [--timeout 5s]
```

### `ang config doctor`

Validate runtime env against generated `internal/config/config.go`.

```bash
ang config doctor [--config-path internal/config/config.go] [--env-file .env] [--example-file .env.example]
```

## API / DB / Contracts

### `ang api-diff`

Compare OpenAPI baseline vs current and print semver recommendation.

```bash
ang api-diff [--base api/openapi.base.yaml] [--current api/openapi.yaml]
ang api-diff --write-base
```

### `ang db`

Database schema utility commands.

```bash
ang db status
ang db sync
```

### `ang migrate`

Atlas migration workflow.

```bash
ang migrate diff add_users_table
ang migrate apply
```

Environment:

- `DB_URL` for `migrate apply`
- `DATABASE_URL` (or `DB_URL`) for `db status` / `db sync`

### `ang contract-test`

Run generated contract tests.

```bash
ang contract-test
```

### `ang test gen`

Generate flow-derived test cases.

```bash
ang test gen [projectPath] [--out tests/generated/flow_cases.json]
```

## Architecture Audit

### `ang vet`

Architecture invariants check.

### `ang vet logic`

Validate embedded Go snippets in CUE.

### `ang rbac`

RBAC introspection.

```bash
ang rbac actions
ang rbac inspect
```

### `ang events map`

Publisher/subscriber map for domain events.

## Tooling

### `ang explain <CODE>`

Explain lint/diagnostic codes with examples.

### `ang draw`

Generate architecture diagrams (Mermaid).

### `ang hash`

Hash summary for inputs/templates/artifacts.

```bash
ang hash
ang hash --artifacts
```

### `ang lsp --stdio`

Run MVP language server.

### `ang mcp`

Run ANG MCP server over stdio.
