# ANG migration notes — July 2026

## Generated output safety

- Failed builds now roll back generator-owned backend, SDK, scripts, tests, cache, and configured frontend destinations.
- `templateHash` fingerprints templates embedded in the running binary. Delete stale manifests created with the empty SHA-256 value once; the next successful build recreates them.
- Missing or locally drifted generated files are emitted again; clearing the whole `.ang/cache` directory is no longer required.

## Validation

- Embedded Go implementations validate `req`, `resp`, and implicit `out` DTO selectors before emission. Unknown fields fail with `DTO_FIELD_UNKNOWN` and a suggested field name.
- Go implementations can move to sidecar files with `impls.go.funcRef: "path.go#function"`.
- Warnings support `//ang:nolint CODE` on the source line or either of the two preceding lines.

## Runtime behavior

- NATS service subscriptions use queue delivery by default. Use `{op: "Handler", delivery: "broadcast"}` when every replica must receive the event.
- `opaque_session_cookie` generation now fails if Redis, session-store, or refresh-store DI wiring is incomplete.

## CLI and CI

- Breaking OpenAPI operation removals fail generation. Use `--accept-contract` only after reviewing the removal.
- `ang build --json` emits structured diagnostics; non-TTY builds select JSON mode automatically.
- `ang doctor` audits the current project when no build log exists; `ang doctor --code CODE` prints targeted guidance.
- `ang up --frontend` starts the frontend dev server and writes `.ang/frontend.pid` and `.ang/frontend.log`.
