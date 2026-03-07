# Java Parsing Improvements Roadmap

This document defines a practical, implementation-oriented plan to improve Java -> IR -> CUE translation quality for ANG.

## Goal

Maximize deterministic extraction from real-world Java projects while minimizing hallucination and silent data loss.

Success criteria:

- Higher endpoint/entity/security extraction coverage.
- Lower `low_confidence` and `unmapped` counts in import reports.
- Fewer manual post-import edits in CUE.

## Core principle: normalize first

Use one canonical pipeline:

`sources -> extractors -> unified IR -> conflict resolver -> CUE renderer`

Do not render CUE directly from source-specific extractors. All source adapters must produce the same IR shape.

## What to extract from Java projects

Minimum complete coverage for enterprise Spring-style services:

1. HTTP surface
- Controllers and route handlers (`@RestController`, `@Controller`, `@RequestMapping`, method mappings).
- JAX-RS and Micronaut equivalents.
- Path/query/header/body parameter contracts.
- Response wrappers (`ResponseEntity<T>`, `Page<T>`, `Slice<T>`, `Optional<T>`, custom wrappers).

2. Domain/data layer
- JPA entities, embeddables, enums.
- Relations (`@OneToMany`, `@ManyToOne`, `@ManyToMany`, `@OneToOne`).
- Composite keys (`@EmbeddedId`, `@IdClass`).
- Cascade/fetch intent and soft-delete markers.
- SQL migrations (Flyway/Liquibase) as schema evidence.

3. Validation
- Bean Validation annotations (`@NotNull`, `@Size`, `@Pattern`, `@Min`, `@Max`, `@Email`, custom validators).
- Requiredness and bounds mapped to CUE constraints.

4. Security and authz
- `@PreAuthorize`, `@Secured`, role/scope expressions.
- Auth type hints and tenant restrictions when detectable.
- Required guard order for generated flow:
  `auth -> validate -> rate/idempotency -> business`.

5. Error contract
- `@ControllerAdvice` and exception handlers.
- Mapped status codes and typed error payload contracts.
- RFC7807/ProblemDetails style responses when present.

6. Constants and semantic symbols
- Java constants and enums used in branching/status logic.
- Preserve symbol names and values in IR.
- Mark unresolved symbol uses explicitly instead of guessing values.

7. Behavioral call graph hints
- Service-to-repository and service-to-client call references.
- External side-effect points (HTTP, queue, storage, mail, events).
- Flag multi-effect chains for Saga/compensation recommendations.

## Recommended architecture

1. Source adapters (independent)
- `openapi`
- `spring-annotations`
- `jax-rs`
- `micronaut`
- `webflux`
- `jpa-hibernate`
- `flyway-liquibase-sql`
- `bytecode` (fallback)
- `mapstruct`

2. Unified IR model
- Stable entities: endpoint, operation, request/response, validation, security, error contract, persistence, events, constants.
- Every artifact carries provenance:
  - extractor
  - source file/line
  - evidence snippets
  - confidence (`high|medium|low`)

3. Deterministic merge with explicit conflicts
- Precedence baseline:
  `OpenAPI > annotations > code inference > DB`.
- Never silently overwrite on conflict.
- Emit machine-readable conflict records with chosen source and rejected alternatives.

4. CUE contract renderer (contract layer only)
- Update schema/http/contracts by default.
- Do not overwrite hand-written flow/impl sections unless explicitly requested.

## Parsing quality upgrades (most impactful first)

1. Build a project symbol table
- Resolve imports, package names, nested classes, aliases, and static imports.
- Track generic substitutions (`T`, `Page<T>`, wrappers).

2. Method signature normalization
- Normalize route method + path + media types.
- Normalize input/output model references with resolved generic types.

3. Expression-level extraction for constants
- Detect enum/constant comparisons in conditional branches.
- Capture known status/state machines into IR enums or decision hints.

4. Error-path extraction
- Map thrown exceptions to handler outputs and status codes.
- Attach fallback/typed error outcomes to operations.

5. Security expression parsing
- Parse common SpEL patterns (`hasRole`, `hasAuthority`, scope checks).
- Convert to normalized RBAC requirement nodes in IR.

6. Validation composition
- Merge constraints from DTO fields, method params, and custom validators.
- Detect conflicting constraints and emit report warnings.

7. Confidence scoring by evidence quality
- `high`: multi-source agreement or explicit schema/annotation evidence.
- `medium`: partial evidence with consistent type signal.
- `low`: inferred only; must include TODO and fix hint.

## Verification loop requirements

Add a repeatable parity loop after import:

1. Runtime snapshot checks
- Compare extracted endpoints to runtime OpenAPI (if available).
- Compare request/response signatures and status codes.

2. Contract tests integration
- Optional command hook (`--contract-test-cmd`) for project-specific checks.
- Include pass/warn/fail in import report.

3. Drift reporting
- Show additions/removals/shape mismatches since last successful snapshot.

## How to avoid hallucinations during translation

- Generate only from extracted facts and known action schemas.
- If evidence is insufficient:
  - emit `unknown`/`openQuestion`/TODO in report,
  - keep confidence low,
  - never fabricate missing fields or constraints.

## Handler coverage rule of thumb

For each Java project, the importer should enumerate and classify all externally reachable handlers:

- HTTP handlers
- message/queue consumers
- scheduled jobs
- gRPC service methods
- webhook entry points

Any unclassified handler must appear in `mapping.unmapped[]` with a concrete reason and fix hint.

## Testing strategy (required for production quality)

1. Unit tests
- Parser edge cases per framework and annotation family.
- Conflict resolution deterministic behavior.

2. Golden tests
- End-to-end Java fixtures -> stable IR/CUE snapshots.

3. Fuzz tests
- Annotation tokenization and nested generic parsing.
- Route/path normalization.
- Type mapping and wrapper unwrapping.

4. Regression suites
- Real-world sample apps (Spring Petclinic + mixed-framework fixtures).
- CI gate on coverage and confidence thresholds.

## Immediate next implementation steps

1. Add symbol-table pass shared by all Java extractors.
2. Extend import report with per-artifact provenance/evidence blocks.
3. Add handler coverage metrics (`total`, `classified`, `unmapped`).
4. Add constants/enums usage map with unresolved symbol warnings.
5. Add runtime verification diff summary in `--report`.
