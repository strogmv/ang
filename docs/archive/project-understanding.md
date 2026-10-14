# Project Understanding Guide

This document is the fast path to understand ANG before writing or changing documentation.

## 1. What ANG is

ANG is an intent compiler:

`CUE intent -> parser -> normalizer -> flowsem -> IR -> emitters -> generated artifacts`

The source of truth is `cue/`. Generated outputs (`internal/`, `api/`, `sdk/`, `db/*`) are build artifacts.

## 2. Repository boundaries

## Core repository (`ang`)

- `cmd/ang`: CLI surface (`build`, `validate`, `import`, `extract`, `explain`, etc.)
- `compiler/*`: parser/normalizer/flow semantics/IR/emitter pipeline
- `cue/*`: architectural intent
- `docs/*`: contracts and operational guides

## Transformation repository (`ang-transform`)

- Shared extraction/normalization for import workflows
- Consumed from `ang` via Go module dependency
- In current workspace:
  - dependency: `github.com/strogmv/ang-transform v0.0.0`
  - local replace: `replace github.com/strogmv/ang-transform => ../ang-transform`

## 3. Import pipeline mental model (Java/OpenAPI/SQL/proto/bytecode)

`ang import java` runs a multi-source merge into a single internal IR and then renders CUE contract files.

Main stages:

1. Extract source facts (Java annotations, OpenAPI, SQL, proto/grpc, bytecode fallback).
2. Merge with fixed priority:
   `OpenAPI > annotations > code inference > DB`.
3. Emit conflict report (never silently drops conflicts).
4. Compute confidence and TODOs for uncertain artifacts.
5. Run optional verification loop (`--verify`, `--verify-openapi`, `--contract-test-cmd`).
6. Render contract-layer CUE files (`project/entities/operations/contracts`).

## 4. What to read first (for accurate docs)

1. `README.md` (product position + architecture overview)
2. `docs/architecture.md` (compiler stages)
3. `docs/commands.md` (CLI contract)
4. `cmd/ang/import.go` (user-facing import behavior)
5. `cmd/ang/extract.go` (facts extraction surface)
6. `compiler/flowsem/*` and `compiler/emitter/service_flow_codegen*` (Flow behavior)
7. `../ang-transform/pkg/transform/*` (language/source extractors and merge logic)

## 5. Documentation writing order

Use this order to avoid drift and contradictions:

1. Contracts first:
   - command syntax
   - input/output schemas
   - merge/priority rules
2. Runtime behavior second:
   - verification loop
   - confidence model
   - conflict and TODO semantics
3. Examples last:
   - sample `ang import java ... --report --verify`
   - sample `ang extract --from=java|proto|bytecode`

## 6. Definition of done for documentation updates

- Every CLI behavior described in docs is reproducible from current code.
- All non-trivial claims are tied to a command or file path.
- Import docs explicitly describe:
  - supported sources/adapters,
  - merge precedence,
  - confidence levels,
  - conflict report semantics,
  - verification checks and statuses (`pass|warn|fail|skip`).

## 7. Minimal verification checklist before publishing docs

Run:

```bash
go test ./cmd/ang
go test ./...
go run ./cmd/ang import java <project> --report --verify
```

For `ang-transform` fuzz smoke:

```bash
GOCACHE=/tmp/go-build go test ./pkg/transform -run=^$ -fuzz=FuzzSplitTopLevelNoPanic -fuzztime=6s
GOCACHE=/tmp/go-build go test ./pkg/transform -run=^$ -fuzz=FuzzJavaTypeToCUENoPanic -fuzztime=3s
GOCACHE=/tmp/go-build go test ./pkg/transform -run=^$ -fuzz=FuzzNormalizeHTTPPathNoPanic -fuzztime=3s
```
