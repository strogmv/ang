# Runtime Config Guide

ANG generates runtime config schema in:

- `internal/config/config.go`
- `.env.example`

Use this as the source of truth for required runtime values.

## Resolution Order

For `ang config doctor`, value resolution follows:

1. Process environment (`os.Getenv`)
2. `.env` file
3. `env-default` tag value from `internal/config/config.go`

## Validate Config

```bash
ang config doctor
```

Custom paths:

```bash
ang config doctor \
  --config-path internal/config/config.go \
  --env-file .env \
  --example-file .env.example
```

## Startup Preflight

`ang doctor start` includes config checks by default:

```bash
ang doctor start
```

Skip only when needed:

```bash
ang doctor start --skip-config
```

## Conditional Requirements

Some fields become required depending on other values:

- `JWT_ALG=RS256|ES256` -> requires `JWT_PUBLIC_KEY`
- `JWT_ALG=HS256` -> requires `JWT_PRIVATE_KEY`
- `EMAIL_PROVIDER=smtp` -> requires SMTP fields (`SMTP_HOST`, `SMTP_FROM`, `SMTP_USER`)

Use `ang config doctor` after changing JWT or mailer strategy.

## Recommended Local Setup

1. Copy `.env.example` to `.env`.
2. Fill secrets and external endpoints.
3. Run `ang config doctor`.
4. Run `ang doctor start`.
5. Run `ang up`.

## Best Practices

- Keep `.env.example` complete but non-secret.
- Never commit real secret values.
- Prefer explicit values for production; avoid relying on defaults silently.
- Re-run `ang config doctor` after every `ang build` when config schema changes.
