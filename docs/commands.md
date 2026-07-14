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
ang up [--project-path .] [--frontend] [--compose-file docker-compose.yml] [--skip-doctor] [--skip-compose] [--skip-build] [--skip-smoke]
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
ang build [--mode in_place|release] [--backend-dir .] [--target go] [--dry-run] [--log-format json] [--accept-contract]
```

Planning mode:

```bash
ang build --phase plan --out-plan .ang/build.plan.json --json
ang build --phase apply --plan-file .ang/build.plan.json
```

## Import & Extraction

### `ang import java`

Import Java/OpenAPI/SQL/proto/bytecode evidence into normalized contract-layer CUE.

```bash
ang import java [path] [--report] [--report-out report.json] [--diff] [--update] [--out-dir cue/import] [--java-parser auto|regex|treesitter|antlr]
```

Common flags:

- `--out-dir`: output directory for generated contract CUE files
- `--report`: print JSON import report to stdout
- `--report-out`: write report JSON to a file
- `--diff`: show file-level diff against current output directory
- `--update`: write generated files to disk
- `--incremental`: process only changed files from git working tree
- `--java-parser`: Java parser backend (`auto|regex|treesitter|antlr`)
  - `auto` currently resolves to `treesitter`.
  - default is `auto`.
- `--profile`: `layered|hexagonal|legacy_monolith|microservice`
- `--verify`: run verification checks
- `--verify-openapi`: explicit OpenAPI snapshot path for parity check
- `--contract-test-cmd`: command for runtime contract tests

Examples:

```bash
ang import java /path/to/project --report
ang import java /path/to/project --diff --update --report-out import-report.json
ang import java /path/to/project --verify --verify-openapi src/main/resources/openapi.yml --contract-test-cmd "mvn -q test"
```

### `ang import openapi`

Import OpenAPI as first-class contract source and generate canonical API/domain CUE.

```bash
ang import openapi path/to/openapi.yml [--report] [--diff] [--update]
```

Common flags:

- `--out-api-dir`: target API directory (`cue/api` by default)
- `--out-domain-file`: target domain entities file (`cue/domain/entities.cue` by default)
- `--group-by-owner`: split operations into `operations_<owner>.cue`
- `--report`, `--report-out`, `--diff`, `--update`

### `ang extract`

Extract normalized facts JSON from source code/spec/schema inputs.

```bash
ang extract [path] [--from auto|go|java|proto|grpc|bytecode|openapi|sql] [--java-parser auto|regex|treesitter|antlr] [--out facts.json]
```

Examples:

```bash
ang extract /path/to/project --from java
ang extract /path/to/project --from proto --out facts-proto.json
ang extract /path/to/project --from bytecode --out facts-bytecode.json
```

## Startup Diagnostics

### `ang doctor`

Analyze build logs and suggest CUE fixes.

```bash
ang doctor [--log-file ang-build.log]
ang doctor --stdin < ang-build.log
ang doctor --log "inline text"
ang doctor --project-path .
ang doctor --code DTO_FIELD_UNKNOWN
```

Without a readable build log, `doctor` checks the current project semantics,
generated artifact hashes, dead CUE configuration, and generated DI wiring.

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

### `ang advise`

Read-only expert audit built from existing compiler diagnostics.

```bash
ang advise --goal project.audit
ang advise --goal project.audit --json
ang advise --goal project.audit --facts facts.json --pack cue/expert/security --json
```

`--json` writes the versioned `ang/expert-report/v1` envelope. The current
goal is only `project.audit`: it creates findings, diagnostics and explanation
traces with `origin: "compiler"`, but never writes CUE, proposes patches or
accepts `--apply`.

`--facts` accepts an `ang/facts/v1` document from `ang extract`; it is
canonicalized and reported as `facts_hash`. Each repeatable `--pack` points to
a CUE directory with a top-level `pack` constrained by
`schema.#ExpertKnowledgePack`. Packs require `--facts` and can add only
deterministic findings/traces in the current phase.

If facts conflict, or mutually exclusive rules with the same `conflict_key`
match, the report contains conflict findings and its status becomes `blocked`.
Resolve that evidence or rule-pack conflict before treating the report as
advice.

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

### `ang explain <CODE|error-json|path-to-json>`

Explain diagnostics in AI-friendly structured form.

```bash
ang explain MISSING_OUTPUT
ang explain --json MISSING_OUTPUT
ang explain build-errors.json --json
ang lint --json | ang explain - --json
```

Output schema (`--json`):
- `schema` (`ang/explain/v2`)
- `items[].code`
- `items[].path`
- `items[].expected`
- `items[].found`
- `items[].hint`
- `items[].doc_anchor`

### `ang actions`

Machine-readable flow action catalog (source-of-truth from `flowsem`).

```bash
ang actions --json
ang actions --cue
```

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
