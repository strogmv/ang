# Flow Semantics

This document describes how `impl_steps`/flow is validated and emitted.

Primary implementation:

- Validation: `compiler/flowsem`
- Emission: `compiler/emitter/service_flow_codegen_*`

## Flow Step Shape

Each normalized step has:

- `action` (string, required)
- `args` (map-like arguments)
- `children` (nested blocks such as `_then`, `_do`, `_fallback`)
- source position metadata (`file`, `line`, `column`, `cuePath`)

## Validation Model

`flowsem` enforces:

- known action families/prefixes
- required args per action
- required children per control action
- custom constraints (timeouts, retries, options, branch presence, etc.)
- transaction-only action placement (e.g., actions requiring `tx.Block`)

Failures are emitted as structured issues with:

- stable code (`MISSING_*`, `UNKNOWN_ACTION`, `TX_REQUIRED`, ...)
- human hint
- location back to CUE source

## Control Flow Actions

Supported control constructs include:

- `flow.If` with `_then` and optional `_else`
- `flow.For` with `_do`
- `flow.While` with `_do`
- `flow.Switch` with `_cases` and optional `_default`
- `flow.Block`, `tx.Block`
- resilience set: `flow.Try`, `flow.Catch`, `flow.Retry`, `flow.Timeout`, `flow.Fallback`

## Scope Semantics

Emitter uses branch-local state cloning for nested control blocks, which prevents accidental variable leakage across sibling branches.

Practical effect:

- variables introduced in one branch do not implicitly pollute unrelated branches
- emitted Go keeps declarations deterministic and avoids hidden re-declaration conflicts

## Collection and Utility Semantics

Examples:

- `list.Filter` emits copy-on-filter pattern (`[:0:0]`) to avoid shared backing-array corruption.
- `list.Paginate` clamps offset/limit boundaries and emits deterministic slicing.
- `flow.SuggestNext` supports string/`[]string` options and can assign output or log.

## Resilience Semantics

- `flow.Try` captures branch-local state and stores `_flowLastError`.
- `flow.Retry` supports bounded retry and optional backoff.
- `flow.Timeout` enforces duration contract and wraps execution with timeout behavior.
- `flow.Fallback` executes fallback branch when primary branch fails.

## Action Catalog

Core action families:

- `repo.*`, `mapping.*`, `logic.*`, `event.*`
- `flow.*`, `tx.*`, `list.*`, `str.*`, `time.*`, `json.*`, `http.*`
- `cache.*`, `mail.*`, `storage.*`, `queue.*`, `webhook.*`

For domain-level reusable patterns see:

- [Universal Flow Actions](./universal-flow-actions.md)

## repo.Query — Custom Repository Methods

`repo.Query` calls arbitrary named repository methods with single or multiple arguments.

### Single-arg form
```cue
{ action: "repo.Query", source: "Application", method: "GetByCode",
  input: "req.Code", output: "app", error: "Not Found" }
```

### Multi-arg form (added v0.1.113)
```cue
{ action: "repo.Query", source: "Application", method: "GetByTenderAndCompany",
  args: ["req.TenderID", "req.CompanyID"], output: "app", error: "Not Found" }
```
- `args: [...]` — joined as comma-separated args after `ctx`
- `list: true` — marks output as slice (`[]domain.X`), skips nil check

Emits: `output, err := s.SourceRepo.Method(ctx, arg1, arg2, ...)`

## crypto.Hash — String Hashing

```cue
{ action: "crypto.Hash", input: "req.Password", output: "hashed" }               // SHA-256 hex
{ action: "crypto.Hash", algo: "bcrypt", input: "req.Password", output: "hashed" }  // bcrypt cost=12
```

- Default algo: `sha256` — emits `sha256.Sum256` + `hex.EncodeToString`
- `bcrypt` — emits `bcrypt.GenerateFromPassword([]byte(input), 12)`
- `output` type: `string`

## math.Expr — Verbatim Arithmetic

```cue
{ action: "math.Expr", expr: "req.Price * 0.9", output: "discounted", declare: true }
{ action: "math.Expr", expr: "total / float64(count)", output: "avg" }
```

- `declare: true` → `output := expr` (declares new variable)
- Without declare → `output = expr` (assigns to existing variable)
- `expr` is emitted verbatim as Go code — use standard Go arithmetic operators

## Authoring Rules (Recommended)

1. Always specify required args explicitly, even if obvious.
2. Keep branch payloads short and composable.
3. Use named outputs for values reused across steps.
4. Wrap transactional mutations in `tx.Block`.
5. Prefer explicit fallback/retry contracts over implicit error swallowing.

## Testing Flow Changes

When adding or changing actions:

1. `go test ./compiler/flowsem -count=1`
2. `go test ./compiler/emitter -count=1`
3. `go test ./compiler/... -count=1`
4. end-to-end `ang build` smoke in generated project
