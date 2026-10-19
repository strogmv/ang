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

### `ang schema sync|check`

```bash
ang schema sync [project]    # vendor the pinned ANG's CUE schema into cue.mod/pkg
ang schema check [project]   # exit 1 when the vendored schema differs (--json)
```

Projects import `github.com/strogmv/ang/cue/schema`; see docs/schema.md.

### `ang lint`

Semantic linting and optional test-coverage checks.

```bash
ang lint [--json] [--check-test-coverage] [--test-dir tests] [--min-coverage 80] [--generate-stubs]
```

### `ang build`

Compile CUE intent into code and artifacts.

```bash
ang build [--mode in_place|release] [--backend-dir .] [--target go] [--dry-run] [--log-format json] [--accept-contract] [--accept-removals] [--release-ref HEAD]
```

A build that removes methods from a generated repository interface
(`internal/port/*Repository`) that the previous generation had is refused and
lists them; pass `--accept-removals` when the finders were deleted on purpose.
A mass removal usually means a file of `cue/repo` is broken.

A build whose OpenAPI contract (`api/openapi.yaml`) breaks a client of the
previous generation is refused with `CONTRACT_BREAKING_CHANGE`, one change per
line, and writes nothing; it is refused again on every run until
`--accept-contract`, after which the new contract is the baseline. Breaking
is: a removed operation, schema or field; a changed field type; a field a
request must now carry (new required, or optional → required); a field a
response no longer promises (required → optional). A response that gains
fields, required or not, breaks no client. Schemas are judged by where the
operations use them (request body or parameters, response, both); one no
operation references by its name (`<Op>Request` — GET inputs are query
parameters —, `<Op>Response`), else both ways. `--dry-run` and `--check` apply
both gates too, so they fail where the real build would.

Both gates only guard what was released. The release baseline is the
generated files as committed at `--release-ref` (default `$ANG_RELEASE_REF`,
else `HEAD`): a change that touches only an operation, schema or field — or a
repository method — that this revision does not have passes, with a note
(`… (unreleased), not gated`). So adding an operation, building, and changing
its output again before the commit needs no flag. Name the branch or tag that
is deployed (`--release-ref main`, `--release-ref v1.4.0`) to treat everything
newer as unreleased; `--release-ref none` compares with the previous
generation only. Outside a git work tree, or when the revision is unknown, the
previous generation is the baseline.

The build first vendors ANG's CUE schema into
`cue.mod/pkg/github.com/strogmv/ang/cue/schema` when it differs; `--dry-run`
only warns and `--check` lists those files among the differences
(docs/schema.md).

Planning mode:

```bash
ang build --phase plan --out-plan .ang/build.plan.json --json
ang build --phase apply --plan-file .ang/build.plan.json
```

### `ang fix dto`

Rewrites the old DTO marker. A `_dto: true` field of a top-level declaration
is removed. The declaration gets `@dto(only="true")` if it does not have it.

```bash
ang fix dto [--check] [--json] [--path cue]
```

The attribute is the only DTO marker. A DTO is a shape that nothing stores,
so it gets no table and no repository. An attribute stays on the declaration
it is written on. A hidden `_dto` field would pass to every entity that embeds
the declaration, and ANG does not read it at all. A declaration that had only
`_dto: true` becomes a DTO here, and the command lists it, because its
generated code changes. A `_dto` field anywhere else is left as it is and
listed. `ang validate` warns about each `_dto` field (`DTO_MARKER_DEPRECATED`).
It also warns about a struct entity that has no `id` and no marker, which gets
no table (`ENTITY_WITHOUT_ID`).

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

Compare a database with `db/schema/schema.sql`. `status` lists what the
database lacks; `sync` applies only additive, idempotent statements and prints
a warning for each drop or type change it leaves alone.

```bash
ang db status
ang db sync
```

### `ang migrate`

`diff` writes a migration with additive, idempotent statements only (Atlas
plans, ANG writes; drops and type changes become warnings). See
[sql-schema.md](sql-schema.md).

```bash
ang migrate diff add_users_table [--dev-url URL] [--dir db/migrations] [--to db/schema/schema.sql] [--dry-run]
ang migrate apply
```

Environment:

- `DB_URL` for `migrate apply`
- `DATABASE_URL` (or `DB_URL`) for `db status` / `db sync`
- `ANG_ATLAS_DEV_URL` overrides the Atlas dev database of `atlas.hcl`

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

```bash
ang vet logic                      # syntax only
ang vet logic --types              # generate into a temp dir and compile
ang vet logic --types --json
ang vet logic --types --packages ./internal/service/...,./cmd/server
```

`--types` generates the project as a dry run, compiles the generated Go with
`go build -overlay` (the project on disk is not touched; unchanged packages come
from the build cache) and prints each compiler error at the CUE line the Go was
written on, followed by the generated location:

```
cue/api/impl_tender_get.cue:46:41: s.TenderProductRepo.ListByTenderNope undefined (…)
    generated: internal/service/tender__get_tender.gen.go:71:37
```

Generated files do not record source lines, so the CUE line is found by its
text (and its neighbours). When the error is in generated scaffolding, or CUE
escapes changed the text, the generated location is printed with the CUE files
the file was generated from. `--packages` defaults to `./internal/service/...`;
`--target` and `--allow-lock-mismatch` work as in `ang build`.

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

### `ang explain action <name>`

One action from the catalog: description, arguments and an example step.

```bash
ang explain action repo.Query
ang explain action repo.Query --json
```

### `ang actions`

Machine-readable flow action catalog (source-of-truth from `flowsem`).

```bash
ang actions --json
ang actions --cue
```

`example` is a real CUE step for every action known to Typed Flow IR, taken from
`compiler/flowir/examples.go`; a test decodes each one with the action's own
decoder, so an example always matches what the compiler accepts. Editor hover
shows the same example.

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

- Diagnostics from the semantic phases on open, change and save.
- On save, embedded Go is type-checked in the background (the `ang vet logic
  --types` engine: dry run + `go build -overlay`, the project is not touched);
  errors appear on the CUE lines with `source: "ang-types"`.
- Hover on a flow action shows its catalog entry and example; inside Go blocks,
  hover on `s.<Name>Repo.<Method>`, `port.<Type>` or `domain.<Type>` shows the
  signature or declaration from the generated `internal/port` and
  `internal/domain`.

### `ang mcp`

Run ANG MCP server over stdio.
