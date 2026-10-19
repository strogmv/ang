# gRPC, proto-first — design (backlog 39)

Status: design, nothing implemented. Written 2026-10-19 for the founder's item 39: keep the gRPC
emitter and extend it so that the `.proto` files stay ours as the source of the contract, with the
Go server and client code generated from them, CUE operations bound to proto services, and a
versioning rule.

## 1. What ANG has today

`framework: "grpc"` on a target grants the `grpc` capability (plus `http`, so chi still runs) and two
steps of `compiler/emitter/go/register.go` (profile `go_legacy`):

- `gRPC Proto` → `api/grpc/service.proto` from the IR (`compiler/emitter/grpc.go`): one package
  `ang.v1`, one `service <Name>Service` per ANG service, one `rpc` per operation, messages from the
  operation input/output.
- `gRPC Transport` → `internal/transport/grpc/server.go`: a placeholder (`Register` is a TODO, no
  grpc-go import) and a README telling you to run protoc by hand.

Its contract is unusable as a published API:

| Problem | Why it matters |
|---|---|
| Field numbers are the CUE field order (`fieldNo++`) | inserting or removing a CUE field renumbers every later field — a silent wire break for every client |
| time, uuid, JSON, enums, entities and nested objects all become `string` | no `google.protobuf.Timestamp`, no enums, no nested messages; clients parse strings |
| one package `ang.v1` for everything, no `reserved`, no options | no versioning, no per-domain packages, no deprecation |
| nothing is served | `Register` does nothing; no interceptors for auth, errors, rate limits |

No consumer uses it (dealingi-back is `framework: "chi"`; grpc/protobuf are only indirect modules in
its go.mod), so it can change freely.

## 2. Goal and constraints

- The `.proto` files are hand-written and reviewed like code: they are the contract clients build
  against. ANG never rewrites them.
- Operations keep living in CUE (flows, auth, rate limits, transactions, events): one service layer
  (`<Service>Impl`) serves HTTP and gRPC alike.
- Go message types and stubs come from the protos with the standard generators' output (clients in
  other languages use `protoc`/`buf` as usual).
- ANG checks the binding at `ang validate` time, with file:line on both sides, like every other gate.
- Versioning is explicit (`pkg.v1`, `pkg.v2`) and guarded by a breaking-change gate that shares the
  release baseline of the OpenAPI gate (`--release-ref`, backlog 51).

## 3. Options

### A. CUE-first, proto generated (fix the existing emitter)

ANG keeps writing `.proto` from CUE, but pins field numbers in a lock file
(`api/grpc/fields.lock`: `Message.field = n`, numbers never reused, removed ones `reserved`), maps
time/enum/nested types properly and generates the server.

- + one source of truth (CUE), no binding to check.
- − contradicts the goal: the contract is generated; a CUE refactor can still change it; proto review
  happens on generated text; per-domain packages, comments and options need CUE syntax of their own.

### B. Proto-first, CUE operations bound to rpcs (recommended)

Protos live in the project (`proto/<domain>/v1/*.proto`, a `buf.yaml` module). ANG parses them in
process (`github.com/bufbuild/protocompile`, pure Go, gives descriptors with source positions) and an
operation names the rpc it implements:

```cue
SubmitQuote: schema.#Operation & {
    service: "trade"
    grpc: { rpc: "dealingi.trade.v1.QuoteService/SubmitQuote" }
    input:  { quoteId: string, lines: [...{sku: string, qty: int}] }
    output: { ok: bool, submittedAt: string }
    flow: [...]
}
```

The CUE input/output stay the operation's own types (the HTTP side and the flow use them). ANG
generates, per bound rpc, the adapter `pb.SubmitQuoteRequest → port.SubmitQuoteRequest`, the call
of `<Service>Impl.SubmitQuote`, and `port.SubmitQuoteResponse → pb.SubmitQuoteResponse`. The
binding check (normalize phase, `GRPC_BINDING_INVALID`, file:line in the CUE and the .proto):

- the rpc exists; one operation per rpc; unary rpcs only in phase 1 (server streaming later, see 6);
- every input field of the operation has a request field of the same name (proto `json_name`, so
  `quote_id` ↔ `quoteId`) and a compatible type; an extra proto field is an error unless the
  operation ignores it explicitly (`grpc: {ignore: ["trace_id"]}`);
- every response field has an output field; output fields the proto lacks are allowed (HTTP only)
  but listed with `-verbose`;
- type rules: `string`↔`string`; `int`↔`int32|int64` (ANG int is Go int: `int32` needs a range
  check in the adapter); `float`↔`double`; `bool`↔`bool`; time (`string` with `@format(datetime)`
  or `time.Time`) ↔ `google.protobuf.Timestamp`; list ↔ `repeated`; inline object ↔ message (by
  shape, recursively); string enum ↔ proto enum (value names compared after stripping the
  `ENUM_NAME_` prefix and `_UNSPECIFIED = 0`); optional CUE field (`x?:`) ↔ proto3 `optional`
  (presence) — the same tri-state question as backlog 47; `map[string]X` ↔ `map<string, X>`.

- + the contract is reviewed as protos, clients use the standard toolchains, CUE keeps the logic.
- − two descriptions of the same request (proto message + CUE input) kept in step by a checker; the
  checker is most of the work.

### C. Proto-first, CUE input/output derived from the proto

As B, but `input`/`output` may be omitted: ANG derives the operation's port types from the
messages (`input: grpc.request`). One description less, but the HTTP/OpenAPI side and the TS SDK
then depend on proto types (proto3 has no `required`; every scalar has a default), which changes
validation semantics of every HTTP operation that opts in. Worth it later for gRPC-only operations;
not as the default.

## 4. Recommendation

Option B, with C allowed per operation later for gRPC-only rpcs. Concretely:

1. **Where the protos live:** `proto/` in the project repository, a buf module (`buf.yaml`,
   `buf.lock` for imports such as `google/protobuf/*.proto`, which protocompile resolves from its
   embedded well-known types). Package per domain and major version: `dealingi.trade.v1`.
   `go_package` = `<module>/internal/transport/grpc/pb/<domain>/v1;<domain>v1`. Published to
   outside clients as the buf module (or the BSR) — the files themselves are the deliverable.
2. **Target config:** `grpc: {proto_root: "proto", listen: ":9090", reflection: true}` on the
   target (and `framework` no longer has to choose between chi and grpc: HTTP stays, gRPC is an
   added listener, `SERVER_ROLE` api serves both).
3. **Generated, never edited** (`ang-generated.txt` like the rest):
   - `internal/transport/grpc/pb/...` — messages, by calling protoc-gen-go's generator in process
     (`google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo` is an importable package; the
     same output as `protoc --go_out`, no external tool, pinned by ANG's go.mod);
   - service descriptors and client stubs: the protoc-gen-go-grpc output, either by `go run`
     of a version pinned in `ang.lock`'s toolchain cache or by an ANG template that reproduces
     its (small, stable) output — decide by measuring both on dealingi-back's CI time;
   - `internal/transport/grpc/<domain>_v1_server.gen.go` — the adapters of section 3B;
   - `internal/transport/grpc/server.gen.go` — the `grpc.Server` with interceptors that reuse the
     HTTP stack: auth (bearer from metadata → the same session/JWT check, `auth.roles`), rate limit
     (`x-rate-limit` of the operation), timeout, operation metrics/tracing (`target.operation_metrics`),
     recovery, request id; errors: ANG's `errors.New(status, code, msg)` → gRPC status by the usual
     HTTP→gRPC table plus `google.rpc.ErrorInfo{reason: code}`, validation errors →
     `google.rpc.BadRequest` field violations;
   - clients: the generated stubs are the Go client; ANG adds nothing for other languages.
4. **Binding:** `grpc: {rpc: "<pkg>.<Service>/<Method>"}` in `cue/schema` `#Operation`; the
   checker of section 3B in the normalize phase, so `ang validate`, the LSP and MCP report it.
5. **Versioning:**
   - Wire rules are proto's: never change a field number or type, `reserved` for removed numbers and
     names, new fields are additive. A breaking change goes into `v2` (new package, new files), with
     `v1` kept until its clients moved; the same CUE operation may bind to `v1` and `v2` rpcs
     (`grpc: {rpcs: [...]}`), each with its adapter.
   - Gate: `ang build` runs a breaking check of the parsed descriptors against the protos at the
     release baseline (`--release-ref`, default HEAD — the rule of backlog 51), in process with the
     descriptor diff (rules as `buf breaking`'s FILE category: removed rpc/message/field, changed
     number, type or label, renamed enum value). `--accept-contract` accepts, as for OpenAPI. Teams
     already on buf can run `buf breaking --against '.git#branch=main'` in CI; ANG's gate does not
     need the buf binary.
   - `option deprecated = true` on an rpc → the bound operation's OpenAPI `deprecated: true` too.
6. **Later:** server-streaming rpcs bound to ANG streaming operations (`chunks`), client streaming
   only if a use appears; gRPC-Gateway is not needed (HTTP is already generated from CUE).
7. **The current emitter:** keep `api/grpc/service.proto` generation only as a one-time scaffold:
   `ang grpc init` writes `proto/<domain>/v1/*.proto` from the CUE operations (with proper types
   and numbers) once, after which the files are the source and the old step is deleted.

## 5. Phases and size

| Phase | Content | Proof |
|---|---|---|
| 1 | protocompile loading of `proto_root`, `grpc.rpc` binding, the checker, `ang grpc init` | unit tests per rule; dealingi-back: init + bind 3 operations, `ang validate` clean |
| 2 | pb generation in process, adapters, server with interceptors, listener and roles | a generated sample serves the rpcs; grpcurl/`go test` client against it; auth/errors/rate limit tests |
| 3 | descriptor breaking gate with the release baseline, deprecation, multi-version binding | gate tests like `TestContractGateLetsUnreleasedChangesPass` |
| 4 | server streaming | streaming sample |

Phase 1 is about a week of the work items above; phases 2–3 another two.

## 6. Questions for the founder

1. Do outside clients exist yet (which languages)? It decides whether the BSR or plain files in the
   repo are the delivery.
2. gRPC on the same process and image as HTTP (second listener, recommended) or a separate role?
3. Should HTTP keep being generated from CUE for operations bound to rpcs, or should those become
   gRPC-only (option C)?
4. Is `buf` acceptable as a developer tool (lint, format), given ANG itself would not need it?
