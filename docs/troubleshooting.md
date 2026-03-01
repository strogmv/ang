# Troubleshooting

This page focuses on the most common failures in local startup and generation.

## `field "JWTPublicKey" is required but the value is not provided`

Cause:

- JWT mode implies required key material, usually `JWT_ALG=RS256` without `JWT_PUBLIC_KEY`.

Fix:

1. Check JWT mode in `.env`.
2. Set required key variable.
3. Run:

```bash
ang config doctor
ang doctor start
```

## `go.mod file not found`

Cause:

- Older project scaffold or wrong working directory.

Fix:

1. Ensure you are in project root.
2. If project is old, add `go.mod` and re-run `ang init` flow for missing bootstrap files.
3. Run `ang doctor start` to verify required files.

## `docker compose is unavailable`

Cause:

- Docker not installed or compose plugin missing.

Fix:

1. Install Docker + Compose plugin.
2. Verify `docker compose version`.
3. Re-run `ang doctor start`.

Temporary workaround:

```bash
ang up --skip-compose
```

## `Smoke FAILED` on `/health` or `/health/ready`

Cause:

- Server not started, wrong `HTTP_PORT`, or readiness dependencies unavailable.

Fix:

1. Check `HTTP_PORT` in `.env`/env.
2. Ensure backend process is running.
3. Run explicit smoke:

```bash
ang smoke --base-url http://localhost:8080
```

## `release mode Go target outputs to ... while root go.mod exists`

Cause:

- Build mode/runtime root mismatch.

Fix:

- For root runtime projects: use `--mode=in_place` (or `build.mode: "in_place"`).
- For release layout: run and build from generated release module directory.

## Build reports `MISSING IMPLEMENTATIONS (Blind Spots)`

Cause:

- Operation contracts exist, but no executable impl flow/snippet for some methods.

Fix:

1. Open referenced CUE path/line in report.
2. Add `impl_steps`, `flow`, or logic snippet.
3. Re-run `ang validate`, `ang lint`, `ang build`.

## Unknown flow action / missing required args

Cause:

- Flow step violates semantic contract.

Fix:

1. Read [Flow Semantics](./flow-semantics.md).
2. Verify required args and child blocks.
3. Use `ang lint` to re-check.

## MCP tools not visible in AI client

Cause:

- MCP server not started or client not connected.

Fix:

1. Start MCP integration (`ang mcp`) in your toolchain.
2. Run MCP bootstrap sequence:
   - `ang_mcp_health`
   - `ang_schema`
   - `ang_validate`
3. Fall back to CLI if MCP is unavailable.
