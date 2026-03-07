# Import Report (`ang import java --report`)

`ang import java --report` returns a machine-readable JSON report with merge decisions, risk signals, and verification checks.

Schema id:

- `schema: "ang/import-report/v1"`

## Top-level fields

- `source_path`: analyzed project root
- `profile`: selected import profile
- `generated_at_utc`
- `adapters[]`: source adapters and detection counters
- `frameworks[]`: framework support status
- `summary`: aggregate counts and confidence totals
- `mapping`: exact/lossy mapping stats + unmapped gaps
- `verification`: optional runtime/snapshot checks
- `conflicts[]`: explicit merge conflicts
- `todos[]`: actionable unresolved items
- `diff[]`: output file status (`added|updated|unchanged`)

## `adapters[]`

Typical adapters:

- `spring-annotations`
- `mapstruct`
- `jpa-hibernate`
- `java-constants-enums`
- `grpc-proto`
- `bytecode`
- `openapi`
- `flyway-liquibase-sql`

Each adapter item contains:

- `name`
- `enabled`
- `detected`
- `note` (optional)

## `frameworks[]`

Typical frameworks:

- `spring-mvc`
- `spring-webflux`
- `quarkus`
- `micronaut`
- `jax-rs`
- `grpc`

Each item contains:

- `name`
- `enabled`
- `note`

## `summary`

- `entities`
- `operations`
- `endpoints`
- `constants`
- `enums`
- `conflicts`
- `todos`
- `high_confidence`
- `medium_confidence`
- `low_confidence`

Confidence is assigned per artifact in merged IR:

- `high`: consistent evidence from stronger sources
- `medium`: partial but acceptable evidence
- `low`: inferred/ambiguous, requires manual check

## `mapping`

- `mapped_exact`
- `mapped_lossy`
- `unmapped[]`

`unmapped[]` entries:

- `artifact`
- `reason`
- `source` (optional)
- `fix_hint` (optional)

Use this block to drive deterministic fixes in automation.

## `conflicts[]`

Conflict entry fields:

- `artifact`
- `field`
- `preferred_source`
- `preferred_value`
- `other_source`
- `other_value`
- `resolution`

Default precedence:

`OpenAPI > annotations > code inference > DB`

## `verification`

Enabled by `--verify`.

Checks usually include:

- `http_endpoint_coverage`
- `openapi_snapshot`
- `contract_tests`

Check fields:

- `name`
- `status`: `pass|warn|fail|skip`
- `details`

## Minimal triage workflow

1. Resolve `conflicts[]` that affect public contracts first.
2. Drive `low_confidence` down by adding explicit annotations/specs.
3. Process `mapping.unmapped[]` with highest business impact.
4. Require `verification.checks` without `fail` before `--update` in CI.
