# Runtime Config Guide

ANG generates runtime config schema in:

- `internal/config/config.go`
- `.env.example`

Use this as the source of truth for required runtime values.

## Configuration in services (`target.service_config`)

By default only the auth service receives the loaded configuration; other
operations that need a setting call `config.Load()` (which reads the
environment and `.env` again on every call) or `os.Getenv`.

```cue
state: target: service_config: true   // add service_config?: bool to a closed #Target
```

turns on, for that target:

- `s.Cfg *config.Config` in every `<Service>Impl`. Constructors keep their
  parameters and set it to `config.Default()` (the auth service: to its `cfg`
  argument); the runtime container then gives every service the configuration
  the server loaded at start, so `s.Cfg` in production is what `cmd/server`
  read, once.
- `config.Default()`: every field at its CUE default (`env-default`), nothing
  read from the environment or `.env`; `config.OrDefault(cfg)`. The test
  container starts from `config.Default()` (it used to be `&config.Config{}`,
  all zero values) and hands its `Config` to every service it builds, so
  `WithConfig(...)` reaches `s.Cfg`.
- `config.Get` with a literal key the config declares (e.g. `"SMTP_HOST"`)
  reads `s.Cfg.SMTPHost` instead of `os.Getenv`.
- `CONFIG_LOAD_IN_OPERATION` (warning): an operation's Go still calls
  `config.Load()`; use `s.Cfg`.

Raw Go in operations reads settings as `s.Cfg.SMTPHost`. A test that needs
other values builds one: `cfg := config.Default(); cfg.EmailDryRun = true`.

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
