# ANG repository

ANG is a code generator: CUE intent → Go backend, SQL schema/migrations, OpenAPI/AsyncAPI and a
TypeScript SDK. This repository is the generator itself, not a generated project.

## Layout

```
cmd/ang/        CLI (`ang build`, `validate`, `fmt`, `migrate`, `mcp`, `lsp`, …) — hand-written
compiler/       pipeline, emitters (compiler/emitter, compiler/emitter/go), targets, sqlmigrate — hand-written
angir/          CUE loading, normalizer, IR, flow semantics — hand-written
templates/      Go templates the emitters render (embedded into the binary)
cue/schema/     the canonical CUE schema projects import as github.com/strogmv/ang/cue/schema;
                embedded in the binary and vendored into a project's cue.mod/pkg (docs/schema.md)
cue/*.cue|md    action/flow references used by tests and docs
tool/mcp/       the MCP server behind `ang mcp` — hand-written
tool/explain/   diagnostic explanations for `ang explain` and MCP — hand-written
examples/blog/  sample project with its own go.mod; everything except cue/ and ang.yaml is generated
docs/           reference docs; docs/tech-debt-backlog.md is the backlog
```

## Rules

- All Go code outside `examples/` is source: edit it directly and cover the change with a test.
- Generated code lives only in `examples/*`. Do not hand-edit it there; change `cue/`, the templates
  or the emitters and regenerate: `go run ./cmd/ang build --project examples/blog`.
- `ang build` refuses to run against this checkout itself; always pass a project directory.
- Do not run `ang build` / `make generate` in a real project (e.g. dealingi-back) to test a change:
  use `ang validate <dir>` or `ang build --check` there, or regenerate `examples/blog`.
- The only target ANG generates is Go (`lang: "go"`, framework `chi` or `grpc`, db `postgres`).

## Checks

```bash
go build ./... && go vet ./... && go test ./... -count=1
make lint                            # scripts/lint.sh: gofmt + staticcheck U1000,SA*,S* (pinned via go run)
go run ./cmd/ang validate examples/blog
(cd examples/blog && go build ./... && go test ./...)
go run ./scripts/determinism_ci      # plan/apply/all determinism on a scratch copy of the sample
```

## MCP

`ang mcp` serves the tools registered in `tool/mcp` (`ang_list_tools` prints them). The common ones:
`ang_mcp_health`, `ang_schema`, `ang_validate`, `ang_dry_run`, `ang_plan`, `ang_do`, `cue_apply_patch`,
`run_preset` (`build`, …), `ang_doctor`, `ang_explain_error`, `ang_search`, `repo_read_symbol`.
