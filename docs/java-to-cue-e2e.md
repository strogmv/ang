# Java -> CUE E2E Workflow

This is the recommended end-to-end flow for migrating Java project contracts into ANG CUE.

## 1. Prepare inputs

Required:

- Java project root (Spring/JAX-RS/Micronaut/Quarkus supported via extractors)

Recommended:

- OpenAPI snapshot (for stronger merge confidence)
- SQL migrations (Flyway/Liquibase)
- proto files / bytecode artifacts if present

## 2. Optional preflight: raw facts

Use facts extraction to inspect what ANG sees before merge:

```bash
ang extract /path/to/java-project --from java --out facts-java.json
ang extract /path/to/java-project --from proto --out facts-proto.json
ang extract /path/to/java-project --from bytecode --out facts-bytecode.json
```

## 3. Run Java import in report mode

```bash
ang import java /path/to/java-project --report --verify --verify-openapi src/main/resources/openapi.yml
```

What to check first in output:

- `summary.low_confidence`
- `conflicts[]`
- `mapping.unmapped[]`
- `verification.checks[]`

## 4. Apply contract-layer update

```bash
ang import java /path/to/java-project --diff --update --out-dir cue/import --report-out import-report.json
```

Generated files (default):

- `cue/import/project.cue`
- `cue/import/entities.cue`
- `cue/import/operations.cue`
- `cue/import/contracts.cue`

## 5. Validate and build

```bash
ang validate
ang build
```

## 6. Iterate until stable

Repeat import after Java/OpenAPI updates:

```bash
ang import java /path/to/java-project --incremental --report --diff --update
```

For CI/CD, gate on:

- no `verification` failures,
- acceptable `low_confidence` threshold,
- reviewed `conflicts[]`.

## Spring Petclinic example

```bash
ang import java /home/strog/work/spring-petclinic-rest \
  --report \
  --verify \
  --verify-openapi /home/strog/work/spring-petclinic-rest/src/main/resources/openapi.yml
```

Then apply:

```bash
ang import java /home/strog/work/spring-petclinic-rest --diff --update --out-dir /tmp/petclinic-cue
```
