# ANG Roadmap

## Status Legend

- `done`: implemented and verified
- `done, needs hardening`: implemented, useful now, but worth tightening
- `next`: concrete near-term work
- `research`: not blocked, but needs design work before implementation

## Completed Foundation

### Week 1: Semantics

- `done` `compiler/effects/logos.go`
  Effect taxonomy, safety tags, canonical action semantics.
- `done` `compiler/flowsem/*`
  Action catalog, effect-aware validation, tx rules, prerequisite enforcement, scope-aware checks.
- `done` `compiler/normalizer/effects.go`
  Effect derivation and validation integrated into normalizer and lint path.

Outcome:
- invalid flows like `openai.Chat` without quota/budget are caught by compiler semantics
- effect/tx/scope rules are enforced centrally, not ad hoc

### Week 2: Handlers CUE

- `done` infra/effects parsing in normalizer
- `done` support for `Handlers`, `TestHandlers`, `Middleware`
- `done` project-side convention for `cue/effects/*` plus `cue/infra/*`

Outcome:
- one declarative layer controls runtime bindings and test bindings
- effect middleware chains are configured from CUE

### Week 3: Generation

- `done` `compiler/emitter/effects_registry.go`
  Generates effect registry and runtime/test profiles.
- `done` `compiler/emitter/effects_middleware.go`
  Generates executable middleware wrappers.
- `done` `compiler/emitter/port_mock_gen.go`
  Auto-generates mocks from port interfaces.
- `done` `compiler/emitter/test_container.go`
  Generates mock-first test container with override options.
- `done` `compiler/emitter/effects.go`
  Unified entrypoint for effect artifacts.

Outcome:
- retry/cache/trace/metrics/log/timeout are generated from effect intent
- tests do not require handwritten mocks
- test bootstrap is generated, not assembled manually

### Week 4: flowfn

- `done` `compiler/flowfn/lexer.go`
- `done` `compiler/flowfn/parser.go`
- `done` `compiler/flowfn/transpiler.go`
- `done` `compiler/flowfn/macros.go`
- `done` `compiler/flowfn/ikhe.go`

Outcome:
- readable flow syntax exists
- macros and fragments are supported
- early semantic validation runs through canonical `flowsem`
- invalid flowfn is rejected before later compiler phases

### LSP Baseline

- `done` `compiler/lsp/*`
  Canonical LSP intelligence layer for hover, completion, flowfn diagnostics.
- `done` `cmd/ang/lsp.go`
  CLI LSP transport wired to compiler/lsp core.

Outcome:
- hover can explain actions from compiler semantics
- completion knows prerequisites and tx rules
- flowfn diagnostics appear in real time

## Done, Needs Hardening

### LSP UX

- `done, needs hardening` completion ranking
  Works and understands prerequisites, but can be improved with better context scoring.
- `done, needs hardening` hover text quality
  Uses catalog data correctly, but can be more curated and concise.
- `done, needs hardening` flow/cue mixed-file awareness
  Current logic is good enough for flowfn-centric editing, but deeper CUE embedding support can be improved.

### Effect Runtime

- `done, needs hardening` middleware runtime
  Works across publisher/storage/state, but more adapters can become effect-aware over time.
- `done, needs hardening` test container sugar
  Core override model exists; domain-specific convenience helpers can be layered on top later.

### Meta Generation

- `done, needs hardening` plan/build/emit/validate/write actions
  The deterministic generation pipeline exists, but still needs more dogfooding and scenario coverage.

## Next

### 1. Dogfood End-to-End in `sendbox`

- wire the new meta actions fully through `sendbox`
- move more AI generation flow to deterministic `plan -> emit -> validate`
- reduce remaining legacy/raw-expression behavior in old sandbox flows

Why:
- this is where correctness pressure reveals gaps fastest

### 2. LSP Editor Quality

- add richer completion snippets from action examples
- add code actions for common effect prerequisite fixes
- improve context extraction inside embedded flow blocks in `.cue`

Why:
- current LSP is semantically correct, but UX can still become much more efficient

### 3. Effect-Aware Refactoring and Explainability

- expose effect graph per operation
- explain why an action is unavailable in editor and CLI
- show derived effect profile for a service or operation

Why:
- this turns semantics into day-to-day engineering leverage

## Research

### Effect Polymorphism

- polymorphic effect capabilities
- generic actions over effect families
- capability-specialized generation without duplicating DSL surface

Why it is research:
- needs a coherent type/capability story, not just more registry rows

### Linear or Affine Resource Tracking

- single-use resources
- explicit ownership/consumption rules
- stronger guarantees around transactions, streams, tokens, approvals

Why it is research:
- this changes the language model, validator, and probably emitter contracts

### Full IDE Platform Layer

- LSP is present, but not yet a full editing platform
- future work can add:
  - semantic rename for actions/vars
  - fragment extraction
  - quick-fix synthesis from diagnostics
  - structural flow visualizations

## Guiding Rule

ANG should keep one source of truth:

- semantics in `flowsem` and `effects`
- language sugar in `flowfn`
- runtime generation in `emitter`
- editor intelligence in `compiler/lsp`

No parallel semantics layers. No editor-only rules. No generated-code-only truth.
