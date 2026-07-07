# {{PROJECT_NAME}} runbook

## Normal development cycle

```bash
ang validate
ang build
ang doctor --project-path .
ang up --frontend
```

Edit application intent under `cue/`. Do not edit generated files under
`internal/`, `api/`, `sdk/`, `db/schema/`, or `db/queries/`; regenerate them.

## Diagnostics

Use `ang build --log-format json` for machine-readable output and
`ang doctor --code <CODE>` for code-specific guidance.

If generation fails, ANG validates a staged tree and leaves the previous
generated tree unchanged. Fix the reported CUE location and run the cycle
again. A missing or locally modified generated artifact is recreated on the
next build.

Breaking OpenAPI changes require explicit review and
`ang build --accept-contract`.
