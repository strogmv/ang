# ang-ir

> Since 2026-09-15 this code lives inside ANG as `github.com/strogmv/ang/angir`
> (history kept). The separate `github.com/strogmv/ang-ir` module is frozen at
> `8fa7434`; change the IR here, in the ANG repository.

`ang-ir` is the extracted `CUE -> normalizer -> IR` core from ANG.

It is intentionally limited to deterministic intent loading.
It does not generate runtime code, servers, SDKs, templates, or target-specific artifacts.

## Scope

Included:
- CUE package loading
- optional `cue/*` domain discovery
- normalization into typed intermediate structs
- conversion into the canonical IR schema
- flow parsing/validation required to preserve flow data in IR
- infra/template/notification extraction required by IR

Excluded:
- emitters
- build/apply pipeline
- generated code
- runtime scaffolding
- server/bootstrap wiring
- target-specific compiler logic

## Public API

```go
result, err := angir.Load(projectPath)
if err != nil {
    panic(err)
}

schema := result.Schema
warnings := result.Warnings
```

Shortcut:

```go
schema, err := angir.LoadSchema(projectPath)
```

## Project Layout Expectations

The loader reads these directories when present:
- `cue/domain`
- `cue/architecture`
- `cue/api`
- `cue/policy` or `cue/policies`
- `cue/repo`
- `cue/events`
- `cue/errors`
- `cue/project`
- `cue/views`
- `cue/infra`
- `cue/effects`

Legacy fallback:
- `cue/main.cue`

## Design Rules

- CUE is the source of truth.
- The library should not invent business operations.
- The loader should not mutate intent with hidden domain heuristics.
- Output should be a faithful IR representation of explicit CUE intent.

## Status

Current package set:
- `parser`
- `normalizer`
- `ir`
- `effects`
- `flowsem`
- `flowfn`
- root facade in `load.go`

Smoke-tested against:
- `testdata/minimal`
- `/home/strog/work/ang/examples/minimal`
